package controller

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/backup"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/auth"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/batch"
	certctrl "github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/certificate"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/deployment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/endpoints"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/galera"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/maxscale"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/pvc"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/rbac"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/servicemonitor"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/sql"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/statefulset"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/docker"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

var (
	testCtx                    = context.Background()
	k8sClient                  client.Client
	testCidrPrefix             string
	testEmulateExternalMdbHost string = "mdb-emulate-external-test.default.svc.cluster.local"
	testLogger                 logr.Logger
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	testLogger = zap.New(
		zap.WriteTo(GinkgoWriter),
		zap.UseDevMode(true),
		zap.Level(zapcore.InfoLevel),
	)
	log.SetLogger(testLogger)

	var err error
	testCidrPrefix, err = docker.GetKindCidrPrefix()
	Expect(err).NotTo(HaveOccurred())
	Expect(testCidrPrefix).NotTo(BeEmpty())

	Expect(mariadbv1alpha1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(monitoringv1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(certmanagerv1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(volumesnapshotv1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	cfg, err := ctrl.GetConfig()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	env, err := environment.GetOperatorEnv(testCtx)
	Expect(err).ToNot(HaveOccurred())

	By("Bootstrapping test environment")
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("../..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		UseExistingCluster:    ptr.To(true),
	}
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	DeferCleanup(testEnv.Stop)

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Controller: config.Controller{
			MaxConcurrentReconciles: 1,
		},
	})
	Expect(err).ToNot(HaveOccurred())
	k8sClient = k8sManager.GetClient()

	client := k8sManager.GetClient()
	scheme := k8sManager.GetScheme()
	galeraRecorder := k8sManager.GetEventRecorderFor("galera")
	replRecorder := k8sManager.GetEventRecorderFor("replication")

	kubeClientset, err := kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())

	disc, err := discovery.NewDiscovery()
	Expect(err).ToNot(HaveOccurred())
	Expect(disc.LogInfo(testLogger)).To(Succeed())

	builder := builder.NewBuilder(scheme, env, disc)
	refResolver := refresolver.New(client)

	conditionReady := condition.NewReady()
	conditionComplete := condition.NewComplete(client)

	backupProcessor := backup.NewPhysicalBackupProcessor(
		backup.WithPhysicalBackupValidationFn(mariadbv1alpha1.IsValidPhysicalBackup),
		backup.WithPhysicalBackupParseDateFn(mariadbv1alpha1.ParsePhysicalBackupTime),
	)

	secretReconciler, err := secret.NewSecretReconciler(client, builder)
	Expect(err).ToNot(HaveOccurred())
	configMapReconciler := configmap.NewConfigMapReconciler(client, builder)
	statefulSetReconciler := statefulset.NewStatefulSetReconciler(client)
	serviceReconciler := service.NewServiceReconciler(client)
	endpointsReconciler := endpoints.NewEndpointsReconciler(client, builder)
	batchReconciler := batch.NewBatchReconciler(client, builder)
	authReconciler := auth.NewAuthReconciler(client, builder)
	rbacReconciler := rbac.NewRBACReconiler(client, builder)
	deployReconciler := deployment.NewDeploymentReconciler(client)
	pvcReconciler := pvc.NewPVCReconciler(client)
	svcMonitorReconciler := servicemonitor.NewServiceMonitorReconciler(client)
	certReconciler := certctrl.NewCertReconciler(client, scheme, k8sManager.GetEventRecorderFor("cert"), disc, builder)

	mxsReconciler := maxscale.NewMaxScaleReconciler(client, builder, env)
	replConfig := replication.NewReplicationConfig(client, builder, secretReconciler, env)
	replicationReconciler, err := replication.NewReplicationReconciler(
		client,
		replRecorder,
		builder,
		replConfig,
		replication.WithRefResolver(refResolver),
		replication.WithSecretReconciler(secretReconciler),
		replication.WithServiceReconciler(serviceReconciler),
	)
	Expect(err).ToNot(HaveOccurred())
	galeraReconciler := galera.NewGaleraReconciler(
		client,
		kubeClientset,
		galeraRecorder,
		env,
		builder,
		galera.WithRefResolver(refResolver),
		galera.WithConfigMapReconciler(configMapReconciler),
		galera.WithServiceReconciler(serviceReconciler),
	)

	podReplicationController := NewPodController(
		"pod-replication",
		client,
		refResolver,
		NewPodReplicationController(
			client,
			replRecorder,
			builder,
			refResolver,
			replConfig,
		),
		[]string{
			metadata.MariadbAnnotation,
			metadata.ReplicationAnnotation,
		},
	)
	podGaleraController := NewPodController(
		"pod-galera",
		client,
		refResolver,
		NewPodGaleraController(client, galeraRecorder),
		[]string{
			metadata.MariadbAnnotation,
			metadata.GaleraAnnotation,
		},
	)

	err = (&MariaDBReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: k8sManager.GetEventRecorderFor("mariadb"),

		Environment:     env,
		Builder:         builder,
		RefResolver:     refResolver,
		ConditionReady:  conditionReady,
		Discovery:       disc,
		BackupProcessor: backupProcessor,

		ConfigMapReconciler:      configMapReconciler,
		SecretReconciler:         secretReconciler,
		StatefulSetReconciler:    statefulSetReconciler,
		ServiceReconciler:        serviceReconciler,
		EndpointsReconciler:      endpointsReconciler,
		RBACReconciler:           rbacReconciler,
		AuthReconciler:           authReconciler,
		DeploymentReconciler:     deployReconciler,
		PVCReconciler:            pvcReconciler,
		ServiceMonitorReconciler: svcMonitorReconciler,
		CertReconciler:           certReconciler,

		MaxScaleReconciler:    mxsReconciler,
		ReplicationReconciler: replicationReconciler,
		GaleraReconciler:      galeraReconciler,
	}).SetupWithManager(testCtx, k8sManager, env, ctrlcontroller.Options{MaxConcurrentReconciles: 10})
	Expect(err).ToNot(HaveOccurred())

	err = (&MaxScaleReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: k8sManager.GetEventRecorderFor("maxscale"),

		Builder:        builder,
		ConditionReady: conditionReady,
		Environment:    env,
		RefResolver:    refResolver,
		Discovery:      disc,

		SecretReconciler:         secretReconciler,
		RBACReconciler:           rbacReconciler,
		AuthReconciler:           authReconciler,
		StatefulSetReconciler:    statefulSetReconciler,
		ServiceReconciler:        serviceReconciler,
		DeploymentReconciler:     deployReconciler,
		ServiceMonitorReconciler: svcMonitorReconciler,
		CertReconciler:           certReconciler,

		SuspendEnabled: false,

		RequeueInterval: 5 * time.Second,
		LogMaxScale:     false,
	}).SetupWithManager(testCtx, k8sManager, ctrlcontroller.Options{MaxConcurrentReconciles: 10})
	Expect(err).ToNot(HaveOccurred())

	err = (&BackupReconciler{
		Client:            client,
		Scheme:            scheme,
		Builder:           builder,
		RefResolver:       refResolver,
		ConditionComplete: conditionComplete,
		RBACReconciler:    rbacReconciler,
		BatchReconciler:   batchReconciler,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&PhysicalBackupReconciler{
		Client:            client,
		Scheme:            scheme,
		Recorder:          k8sManager.GetEventRecorderFor("physicalbackup"),
		Builder:           builder,
		Discovery:         disc,
		RefResolver:       refResolver,
		ConditionComplete: conditionComplete,
		RBACReconciler:    rbacReconciler,
		PVCReconciler:     pvcReconciler,
		BackupProcessor:   backupProcessor,
	}).SetupWithManager(testCtx, k8sManager, ctrlcontroller.Options{MaxConcurrentReconciles: 10})
	Expect(err).ToNot(HaveOccurred())

	err = (&RestoreReconciler{
		Client:            client,
		Scheme:            scheme,
		Builder:           builder,
		RefResolver:       refResolver,
		ConditionComplete: conditionComplete,
		RBACReconciler:    rbacReconciler,
		BatchReconciler:   batchReconciler,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	sqlOpts := []sql.SqlOpt{
		sql.WithRequeueInterval(30 * time.Second),
		sql.WithLogSql(false),
	}
	err = NewUserReconciler(client, refResolver, conditionReady, sqlOpts...).SetupWithManager(testCtx, k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = NewGrantReconciler(client, refResolver, conditionReady, sqlOpts...).SetupWithManager(testCtx, k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = NewDatabaseReconciler(client, refResolver, conditionReady, sqlOpts...).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = NewExternalMariaDBReconciler(client, refResolver, conditionReady, builder).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ConnectionReconciler{
		Client:           client,
		Scheme:           scheme,
		SecretReconciler: secretReconciler,
		RefResolver:      refResolver,
		ConditionReady:   conditionReady,
		RequeueInterval:  5 * time.Second,
	}).SetupWithManager(testCtx, k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&SqlJobReconciler{
		Client:              client,
		Scheme:              scheme,
		Builder:             builder,
		RefResolver:         refResolver,
		ConfigMapReconciler: configMapReconciler,
		ConditionComplete:   conditionComplete,
		RBACReconciler:      rbacReconciler,
		RequeueInterval:     5 * time.Second,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = podReplicationController.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = podGaleraController.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&StatefulSetGaleraReconciler{
		Client:      client,
		RefResolver: refResolver,
		Recorder:    galeraRecorder,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = NewWebhookConfigReconciler(
		client,
		scheme,
		k8sManager.GetEventRecorderFor("webhook-config"),
		k8sManager.Elected(),
		testCASecretKey,
		"test",
		3*365*24*time.Hour,
		testCertSecretKey,
		3*30*24*time.Hour,
		33,
		testWebhookServiceKey,
		5*time.Minute,
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(testCtx)
		Expect(err).ToNot(HaveOccurred())
	}()

	By("Creating initial test data")
	testCreateInitialData(testCtx, *env)
	DeferCleanup(func() {
		By("Cleaning up initial test data")
		testCleanupInitialData(testCtx)
	})
})
