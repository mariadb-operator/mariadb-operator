package galera

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/pvc"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/galera/errors"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/pki"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Option func(*GaleraReconciler)

func WithRefResolver(rr *refresolver.RefResolver) Option {
	return func(r *GaleraReconciler) {
		r.refResolver = rr
	}
}

func WithConfigMapReconciler(cmr *configmap.ConfigMapReconciler) Option {
	return func(r *GaleraReconciler) {
		r.configMapReconciler = cmr
	}
}

func WithServiceReconciler(sr *service.ServiceReconciler) Option {
	return func(r *GaleraReconciler) {
		r.serviceReconciler = sr
	}
}

type GaleraReconciler struct {
	client.Client
	kubeClientset       *kubernetes.Clientset
	recorder            record.EventRecorder
	env                 *environment.OperatorEnv
	builder             *builder.Builder
	refResolver         *refresolver.RefResolver
	configMapReconciler *configmap.ConfigMapReconciler
	serviceReconciler   *service.ServiceReconciler
	pvcReconciler       *pvc.PVCReconciler
}

func NewGaleraReconciler(client client.Client, kubeClientset *kubernetes.Clientset, recorder record.EventRecorder,
	env *environment.OperatorEnv, builder *builder.Builder, opts ...Option) *GaleraReconciler {
	r := &GaleraReconciler{
		Client:        client,
		kubeClientset: kubeClientset,
		recorder:      recorder,
		env:           env,
		builder:       builder,
	}
	for _, setOpt := range opts {
		setOpt(r)
	}
	if r.refResolver == nil {
		r.refResolver = refresolver.New(client)
	}
	if r.configMapReconciler == nil {
		r.configMapReconciler = configmap.NewConfigMapReconciler(client, builder)
	}
	if r.serviceReconciler == nil {
		r.serviceReconciler = service.NewServiceReconciler(client)
	}
	if r.pvcReconciler == nil {
		r.pvcReconciler = pvc.NewPVCReconciler(client)
	}
	return r
}

func shouldReconcileSwitchover(mdb *mariadbv1alpha1.MariaDB) bool {
	if mdb.IsMaxScaleEnabled() || mdb.IsRestoringBackup() || mdb.IsUpdating() || mdb.IsResizingStorage() {
		return false
	}
	if mdb.Status.CurrentPrimaryPodIndex == nil {
		return false
	}
	currentPodIndex := ptr.Deref(mdb.Status.CurrentPrimaryPodIndex, 0)
	desiredPodIndex := ptr.Deref(ptr.Deref(mdb.Spec.Galera, mariadbv1alpha1.Galera{}).Primary.PodIndex, 0)
	return currentPodIndex != desiredPodIndex
}

func (r *GaleraReconciler) Reconcile(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (ctrl.Result, error) {
	if !mariadb.IsGaleraEnabled() {
		return ctrl.Result{}, nil
	}
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(mariadb), &sts); err != nil {
		return ctrl.Result{}, err
	}
	logger := log.FromContext(ctx).WithName("galera")

	if mariadb.HasGaleraNotReadyCondition() {
		if result, err := r.reconcileRecovery(ctx, mariadb, logger.WithName("recovery")); !result.IsZero() || err != nil {
			return result, err
		}
	}

	if !mariadb.HasGaleraReadyCondition() && sts.Status.ReadyReplicas == mariadb.Spec.Replicas {
		if err := r.disableBootstrap(ctx, mariadb, logger); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("Galera cluster is healthy")
		r.recorder.Event(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonGaleraClusterHealthy, "Galera cluster is healthy")

		if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
			condition.SetGaleraReady(&mariadb.Status)
			condition.SetGaleraConfigured(&mariadb.Status)
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching Galera status: %v", err)
		}
	}

	if shouldReconcileSwitchover(mariadb) {
		fromIndex := *mariadb.Status.CurrentPrimaryPodIndex
		toIndex := ptr.Deref(ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{}).Primary.PodIndex, 0)

		if err := r.patchStatus(ctx, mariadb, func(status *mariadbv1alpha1.MariaDBStatus) {
			status.UpdateCurrentPrimary(mariadb, toIndex)
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error patching current primary status: %v", err)
		}

		logger.Info("Primary switched", "from-index", fromIndex, "to-index", toIndex)
		r.recorder.Eventf(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonPrimarySwitched,
			"Primary switched from index '%d' to index '%d'", fromIndex, toIndex)
	}
	return ctrl.Result{}, nil
}

func (r *GaleraReconciler) disableBootstrap(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, logger logr.Logger) error {
	logger.V(1).Info("Disabling Galera bootstrap")

	clientSet, err := r.newAgentClientSet(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error creating agent client set: %v", err)
	}
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		agentClient, err := clientSet.clientForIndex(i)
		if err != nil {
			return fmt.Errorf("error creating agent client: %v", err)
		}
		if err := agentClient.Galera.DisableBootstrap(ctx); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("error disabling bootstrap in Pod %d: %v", i, err)
		}
	}
	return nil
}

func (r *GaleraReconciler) newAgentClientSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientOpts ...mdbhttp.Option) (*agentClientSet, error) {
	opts := []mdbhttp.Option{}
	opts = append(opts, clientOpts...)

	agent := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{}).Agent
	kubernetesAuth := ptr.Deref(agent.KubernetesAuth, mariadbv1alpha1.KubernetesAuth{})
	basicAuth := ptr.Deref(agent.BasicAuth, mariadbv1alpha1.BasicAuth{})

	if kubernetesAuth.Enabled {
		opts = append(opts,
			mdbhttp.WithKubernetesAuth(r.env.MariadbOperatorSAPath),
		)
	} else if basicAuth.Enabled && !reflect.ValueOf(basicAuth.PasswordSecretKeyRef).IsZero() {
		password, err := r.refResolver.SecretKeyRef(ctx, basicAuth.PasswordSecretKeyRef.SecretKeySelector, mariadb.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting agent password: %v", err)
		}
		opts = append(opts,
			mdbhttp.WithBasicAuth(basicAuth.Username, password),
		)
	}

	if mariadb.IsTLSEnabled() {
		tlsCA, err := r.refResolver.SecretKeyRef(ctx, mariadb.TLSCABundleSecretKeyRef(), mariadb.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error reading TLS CA bundle: %v", err)
		}

		clientCertKeySelector := mariadbv1alpha1.SecretKeySelector{
			LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
				Name: mariadb.TLSClientCertSecretKey().Name,
			},
			Key: pki.TLSCertKey,
		}
		tlsCert, err := r.refResolver.SecretKeyRef(ctx, clientCertKeySelector, mariadb.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error reading TLS cert: %v", err)
		}

		clientKeyKeySelector := mariadbv1alpha1.SecretKeySelector{
			LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
				Name: mariadb.TLSClientCertSecretKey().Name,
			},
			Key: pki.TLSKeyKey,
		}
		tlsKey, err := r.refResolver.SecretKeyRef(ctx, clientKeyKeySelector, mariadb.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error reading TLS key: %v", err)
		}

		opts = append(opts, []mdbhttp.Option{
			mdbhttp.WithTLSEnabled(mariadb.IsTLSEnabled()),
			mdbhttp.WithTLSCA([]byte(tlsCA)),
			mdbhttp.WithTLSCert([]byte(tlsCert)),
			mdbhttp.WithTLSKey([]byte(tlsKey)),
		}...)
	}

	return newAgentClientSet(mariadb, opts...)
}

func (r *GaleraReconciler) patchStatus(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	patcher func(*mariadbv1alpha1.MariaDBStatus)) error {
	patch := client.MergeFrom(mariadb.DeepCopy())
	patcher(&mariadb.Status)
	return r.Status().Patch(ctx, mariadb, patch)
}
