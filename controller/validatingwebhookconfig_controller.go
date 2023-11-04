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

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	certctrl "github.com/mariadb-operator/mariadb-operator/pkg/controller/certificate"
	"github.com/mariadb-operator/mariadb-operator/pkg/dns"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	wellKnownLabelKey   = "mariadb.mmontes.io/component"
	wellKnownLabelValue = "webhook"
)

type ValidatingWebhookConfigReconciler struct {
	client.Client
	scheme          *runtime.Scheme
	recorder        record.EventRecorder
	certReconciler  *certctrl.CertReconciler
	serviceKey      types.NamespacedName
	requeueDuration time.Duration
	readyMux        *sync.Mutex
	ready           bool
}

func NewValidatingWebhookConfigReconciler(client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder,
	caSecretKey types.NamespacedName, caCommonName string, caValidity time.Duration,
	certSecretKey types.NamespacedName, certValidity time.Duration, lookaheadValidity time.Duration,
	serviceKey types.NamespacedName, requeueDuration time.Duration) *ValidatingWebhookConfigReconciler {

	certDNSnames := dns.ServiceDNSNames(serviceKey)
	return &ValidatingWebhookConfigReconciler{
		Client:   client,
		scheme:   scheme,
		recorder: recorder,
		certReconciler: certctrl.NewCertReconciler(
			client,
			caSecretKey,
			caCommonName,
			certSecretKey,
			certDNSnames.FQDN,
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

func (r *ValidatingWebhookConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var cfg admissionregistration.ValidatingWebhookConfiguration
	err := r.Get(ctx, req.NamespacedName, &cfg)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Unable to get webhook config")
		return ctrl.Result{}, err
	}

	if cfg.Labels[wellKnownLabelKey] != wellKnownLabelValue {
		log.Info("Ignoring webhook due to missing labels", wellKnownLabelKey, wellKnownLabelValue)
		return ctrl.Result{}, nil
	}

	log.Info("Updating webhook config")
	err = r.updateWebhookConfig(ctx, &cfg)
	if err != nil {
		log.Error(err, "Could not update webhook config")
		r.recorder.Eventf(&cfg, v1.EventTypeWarning, mariadbv1alpha1.ReasonWebhookUpdateFailed, err.Error())
		return ctrl.Result{}, err
	}
	log.Info("Updated webhook config")

	r.readyMux.Lock()
	defer r.readyMux.Unlock()
	r.ready = true

	return ctrl.Result{
		RequeueAfter: r.requeueDuration,
	}, nil
}

func (r *ValidatingWebhookConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&admissionregistration.ValidatingWebhookConfiguration{}).
		Complete(r)
}

func (r *ValidatingWebhookConfigReconciler) ReadyCheck(_ *http.Request) error {
	r.readyMux.Lock()
	defer r.readyMux.Unlock()
	if !r.ready {
		return errors.New("Webhook not ready")
	}
	healthy, err := health.IsServiceHealthy(context.Background(), r.Client, r.serviceKey)
	if err != nil {
		return fmt.Errorf("Service not ready: %s", err)
	}
	if !healthy {
		return errors.New("Service not ready")
	}
	return nil
}

func (r *ValidatingWebhookConfigReconciler) updateWebhookConfig(ctx context.Context,
	cfg *admissionregistration.ValidatingWebhookConfiguration) error {
	result, err := r.certReconciler.Reconcile(ctx)
	if err != nil {
		return fmt.Errorf("Error reconciling webhook certificate: %v", err)
	}
	return r.patch(ctx, cfg, func(cfg *admissionregistration.ValidatingWebhookConfiguration) {
		r.inject(ctx, cfg, result.CAKeyPair.CertPEM)
	})
}

func (r *ValidatingWebhookConfigReconciler) inject(ctx context.Context, cfg *admissionregistration.ValidatingWebhookConfiguration,
	certData []byte) {
	log.FromContext(ctx).Info("Injecting CA certificate and service names", "name", cfg.Name)
	for i := range cfg.Webhooks {
		cfg.Webhooks[i].ClientConfig.Service.Name = r.serviceKey.Name
		cfg.Webhooks[i].ClientConfig.Service.Namespace = r.serviceKey.Namespace
		cfg.Webhooks[i].ClientConfig.CABundle = certData
	}
}

func (r *ValidatingWebhookConfigReconciler) patch(ctx context.Context, cfg *admissionregistration.ValidatingWebhookConfiguration,
	patchFn func(cfg *admissionregistration.ValidatingWebhookConfiguration)) error {
	patch := client.MergeFrom(cfg.DeepCopy())
	patchFn(cfg)
	if err := r.Patch(ctx, cfg, patch); err != nil {
		return err
	}
	return nil
}
