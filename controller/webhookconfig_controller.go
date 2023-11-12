/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	certctrl "github.com/mariadb-operator/mariadb-operator/pkg/controller/certificate"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
	"github.com/mariadb-operator/mariadb-operator/pkg/predicate"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type WebhookConfigReconciler struct {
	client.Client
	scheme          *runtime.Scheme
	recorder        record.EventRecorder
	certReconciler  *certctrl.CertReconciler
	serviceKey      types.NamespacedName
	requeueDuration time.Duration
	readyMux        *sync.Mutex
	ready           bool
}

func NewWebhookConfigReconciler(client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder,
	caSecretKey types.NamespacedName, caCommonName string, caValidity time.Duration,
	certSecretKey types.NamespacedName, certValidity time.Duration, lookaheadValidity time.Duration,
	serviceKey types.NamespacedName, requeueDuration time.Duration) *WebhookConfigReconciler {

	certDNSnames := pki.ServiceDNSNames(serviceKey)
	return &WebhookConfigReconciler{
		Client:   client,
		scheme:   scheme,
		recorder: recorder,
		certReconciler: certctrl.NewCertReconciler(
			client,
			caSecretKey,
			caCommonName,
			certSecretKey,
			certDNSnames.CommonName,
			certDNSnames.Names,
			certctrl.WithCAValidity(caValidity),
			certctrl.WithCertValidity(certValidity),
			certctrl.WithLookaheadValidity(lookaheadValidity),
		),
		serviceKey:      serviceKey,
		requeueDuration: requeueDuration,
		readyMux:        &sync.Mutex{},
		ready:           false,
	}
}

func (r *WebhookConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	certResult, err := r.certReconciler.Reconcile(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("Error reconciling webhook certificate: %v", err)
	}

	if err := r.reconcileValidatingWebhook(ctx, req.NamespacedName, certResult); err != nil {
		return ctrl.Result{}, fmt.Errorf("Error reconciling ValidatingWebhookConfiguration: %v", err)
	}

	if err := r.reconcileMutatingWebhook(ctx, req.NamespacedName, certResult); err != nil {
		return ctrl.Result{}, fmt.Errorf("Error reconciling MutatingWebhookConfiguration: %v", err)
	}

	r.readyMux.Lock()
	defer r.readyMux.Unlock()
	r.ready = true

	return ctrl.Result{
		RequeueAfter: r.requeueDuration,
	}, nil
}

func (r *WebhookConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("webhookconfiguration").
		Watches(
			&admissionregistration.ValidatingWebhookConfiguration{},
			&handler.EnqueueRequestForObject{},
		).
		Watches(
			&admissionregistration.MutatingWebhookConfiguration{},
			&handler.EnqueueRequestForObject{},
		).
		WithEventFilter(predicate.PredicateWithAnnotations([]string{
			metadata.WebhookConfigAnnotation,
		})).
		Complete(r)
}

func (r *WebhookConfigReconciler) ReadyHandler(logger logr.Logger) func(_ *http.Request) error {
	return func(_ *http.Request) error {
		r.readyMux.Lock()
		defer r.readyMux.Unlock()
		if !r.ready {
			err := errors.New("Webhook not ready")
			logger.Error(err, "Readiness probe failed")
			return err
		}
		healthy, err := health.IsServiceHealthy(context.Background(), r.Client, r.serviceKey)
		if err != nil {
			err := fmt.Errorf("Service not ready: %s", err)
			logger.Error(err, "Readiness probe failed")
			return err
		}
		if !healthy {
			err := errors.New("Service not ready")
			logger.Error(err, "Readiness probe failed")
			return err
		}
		return nil
	}
}

func (r *WebhookConfigReconciler) reconcileValidatingWebhook(ctx context.Context, key types.NamespacedName,
	certResult *certctrl.ReconcileResult) error {
	logger := log.FromContext(ctx).WithValues("webhook", "validating")
	var validatingWebhook admissionregistration.ValidatingWebhookConfiguration
	if err := r.Get(ctx, key, &validatingWebhook); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	logger.Info("Updating webhook config")
	if err := r.patchValidatingWebhook(ctx, &validatingWebhook, func(cfg *admissionregistration.ValidatingWebhookConfiguration) {
		r.injectValidatingWebhook(ctx, cfg, certResult.CAKeyPair.CertPEM, logger)
	}); err != nil {
		logger.Error(err, "Could not update ValidatingWebhookConfig")
		r.recorder.Eventf(&validatingWebhook, v1.EventTypeWarning, mariadbv1alpha1.ReasonWebhookUpdateFailed, err.Error())
		return err
	}
	logger.Info("Updated webhook config")
	return nil
}

func (r *WebhookConfigReconciler) injectValidatingWebhook(ctx context.Context, cfg *admissionregistration.ValidatingWebhookConfiguration,
	certData []byte, logger logr.Logger) {
	logger.Info("Injecting CA certificate and service names", "name", cfg.Name)
	for i := range cfg.Webhooks {
		cfg.Webhooks[i].ClientConfig.Service.Name = r.serviceKey.Name
		cfg.Webhooks[i].ClientConfig.Service.Namespace = r.serviceKey.Namespace
		cfg.Webhooks[i].ClientConfig.CABundle = certData
	}
}

func (r *WebhookConfigReconciler) patchValidatingWebhook(ctx context.Context, cfg *admissionregistration.ValidatingWebhookConfiguration,
	patchFn func(cfg *admissionregistration.ValidatingWebhookConfiguration)) error {
	patch := client.MergeFrom(cfg.DeepCopy())
	patchFn(cfg)
	if err := r.Patch(ctx, cfg, patch); err != nil {
		return err
	}
	return nil
}

func (r *WebhookConfigReconciler) reconcileMutatingWebhook(ctx context.Context, key types.NamespacedName,
	certResult *certctrl.ReconcileResult) error {
	logger := log.FromContext(ctx).WithValues("webhook", "mutating")
	var mutatingWebhook admissionregistration.MutatingWebhookConfiguration
	if err := r.Get(ctx, key, &mutatingWebhook); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	logger.Info("Updating webhook config")
	if err := r.patchMutatingWebhook(ctx, &mutatingWebhook, func(cfg *admissionregistration.MutatingWebhookConfiguration) {
		r.injectMutatingWebhook(ctx, cfg, certResult.CAKeyPair.CertPEM, logger)
	}); err != nil {
		logger.Error(err, "Could not update MutatingWebhookConfig")
		r.recorder.Eventf(&mutatingWebhook, v1.EventTypeWarning, mariadbv1alpha1.ReasonWebhookUpdateFailed, err.Error())
		return err
	}
	logger.Info("Updated webhook config")
	return nil
}

func (r *WebhookConfigReconciler) injectMutatingWebhook(ctx context.Context, cfg *admissionregistration.MutatingWebhookConfiguration,
	certData []byte, logger logr.Logger) {
	logger.Info("Injecting CA certificate and service names", "name", cfg.Name)
	for i := range cfg.Webhooks {
		cfg.Webhooks[i].ClientConfig.Service.Name = r.serviceKey.Name
		cfg.Webhooks[i].ClientConfig.Service.Namespace = r.serviceKey.Namespace
		cfg.Webhooks[i].ClientConfig.CABundle = certData
	}
}

func (r *WebhookConfigReconciler) patchMutatingWebhook(ctx context.Context, cfg *admissionregistration.MutatingWebhookConfiguration,
	patchFn func(cfg *admissionregistration.MutatingWebhookConfiguration)) error {
	patch := client.MergeFrom(cfg.DeepCopy())
	patchFn(cfg)
	if err := r.Patch(ctx, cfg, patch); err != nil {
		return err
	}
	return nil
}
