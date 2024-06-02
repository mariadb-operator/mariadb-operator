package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	labels "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	"github.com/mariadb-operator/mariadb-operator/pkg/docker"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	stsobj "github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

var (
	testVeryHighTimeout = 5 * time.Minute
	testHighTimeout     = 3 * time.Minute
	testTimeout         = 1 * time.Minute
	testInterval        = 1 * time.Second

	testNamespace = "default"
	testMdbkey    = types.NamespacedName{
		Name:      "mdb-test",
		Namespace: testNamespace,
	}
	testPwdKey = types.NamespacedName{
		Name:      "password",
		Namespace: testNamespace,
	}
	testPwdSecretKey        = "passsword"
	testPwdMetricsSecretKey = "metrics"
	testUser                = "test"
	testDatabase            = "test"
	testConnKey             = types.NamespacedName{
		Name:      "conn",
		Namespace: testNamespace,
	}
	testConnSecretKey = "dsn"
	testCASecretKey   = types.NamespacedName{
		Name:      "test-ca",
		Namespace: testNamespace,
	}
	testCertSecretKey = types.NamespacedName{
		Name:      "test-cert",
		Namespace: testNamespace,
	}
	testWebhookServiceKey = types.NamespacedName{
		Name:      "test-webhook-service",
		Namespace: testNamespace,
	}
)

func testCreateInitialData(ctx context.Context, k8sClient client.Client, env environment.OperatorEnv) {
	var testCidrPrefix, err = docker.GetKindCidrPrefix()
	Expect(testCidrPrefix).ShouldNot(Equal(""))
	Expect(err).ToNot(HaveOccurred())

	password := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPwdKey.Name,
			Namespace: testPwdKey.Namespace,
			Labels: map[string]string{
				metadata.WatchLabel: "",
			},
		},
		Data: map[string][]byte{
			testPwdSecretKey:        []byte("MariaDB11!"),
			testPwdMetricsSecretKey: []byte("MariaDB11!"),
		},
	}
	Expect(k8sClient.Create(ctx, &password)).To(Succeed())

	mdb := mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testMdbkey.Name,
			Namespace: testMdbkey.Namespace,
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
				},
			},
			PodTemplate: mariadbv1alpha1.PodTemplate{
				PodSecurityContext: &corev1.PodSecurityContext{
					RunAsUser: ptr.To(int64(0)),
				},
				PodMetadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"sidecar.istio.io/inject": "false",
					},
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "false",
					},
				},
			},
			Image:           env.RelatedMariadbImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			InheritMetadata: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"k8s.mariadb.com/test": "test",
				},
				Annotations: map[string]string{
					"k8s.mariadb.com/test": "test",
				},
			},
			RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
				SecretKeySelector: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: testPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
			},
			Username: &testUser,
			PasswordSecretKeyRef: &mariadbv1alpha1.GeneratedSecretKeyRef{
				SecretKeySelector: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: testPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
			},
			Database: &testDatabase,
			Connection: &mariadbv1alpha1.ConnectionTemplate{
				SecretName: &testConnKey.Name,
				SecretTemplate: &mariadbv1alpha1.SecretTemplate{
					Key: &testConnSecretKey,
				},
			},
			MyCnf: ptr.To(`[mariadb]
bind-address=*
default_storage_engine=InnoDB
binlog_format=row
innodb_autoinc_lock_mode=2
max_allowed_packet=256M`),
			Port: 3306,
			Service: &mariadbv1alpha1.ServiceTemplate{
				Type: corev1.ServiceTypeLoadBalancer,
				Metadata: &mariadbv1alpha1.Metadata{
					Annotations: map[string]string{
						"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.45",
					},
				},
			},
			Metrics: &mariadbv1alpha1.MariadbMetrics{
				Enabled: true,
				Exporter: mariadbv1alpha1.Exporter{
					Image: env.RelatedExporterImage,
					Port:  9104,
				},
				ServiceMonitor: mariadbv1alpha1.ServiceMonitor{
					PrometheusRelease: "kube-prometheus-stack",
					JobLabel:          "mariadb-monitoring",
					Interval:          "10s",
					ScrapeTimeout:     "10s",
				},
				Username: "monitoring",
				PasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
					SecretKeySelector: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdMetricsSecretKey,
					},
				},
			},
			Storage: mariadbv1alpha1.Storage{
				Size: ptr.To(resource.MustParse("300Mi")),
			},
		},
	}
	applyMariadbTestConfig(&mdb)

	Expect(k8sClient.Create(ctx, &mdb)).To(Succeed())
	expectMariadbReady(ctx, k8sClient, testMdbkey)
}

func testMariadbUpdate(mdb *mariadbv1alpha1.MariaDB) {
	key := client.ObjectKeyFromObject(mdb)

	By("Updating MariaDB compute resources")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, mdb); err != nil {
			return false
		}
		if mdb.Spec.PodTemplate.PodMetadata == nil {
			mdb.Spec.PodTemplate.PodMetadata = &mariadbv1alpha1.Metadata{}

			if mdb.Spec.PodTemplate.PodMetadata.Annotations == nil {
				mdb.Spec.PodTemplate.PodMetadata.Annotations = map[string]string{}
			}
		}
		mdb.Spec.PodTemplate.PodMetadata.Annotations["k8s.mariadb.com/updated-at"] = time.Now().String()

		return k8sClient.Update(testCtx, mdb) == nil
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting MariaDB to be updated eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, mdb); err != nil {
			return false
		}
		return mdb.IsReady() && meta.IsStatusConditionTrue(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeUpdated)
	}, testHighTimeout, testInterval).Should(BeTrue())
}

func testMariadbVolumeResize(mdb *mariadbv1alpha1.MariaDB, newVolumeSize string) {
	key := client.ObjectKeyFromObject(mdb)

	By("Updating storage")
	mdb.Spec.Storage.Size = ptr.To(resource.MustParse(newVolumeSize))
	Expect(k8sClient.Update(testCtx, mdb)).To(Succeed())

	By("Expecting MariaDB to have resized storage eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, mdb); err != nil {
			return false
		}
		return mdb.IsReady() && meta.IsStatusConditionTrue(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeStorageResized)
	}, testHighTimeout, testInterval).Should(BeTrue())

	By("Expecting StatefulSet storage to have been resized")
	var sts appsv1.StatefulSet
	Expect(k8sClient.Get(testCtx, key, &sts)).To(Succeed())
	mdbSize := mdb.Spec.Storage.GetSize()
	stsSize := stsobj.GetStorageSize(&sts, builder.StorageVolume)
	Expect(mdbSize).NotTo(BeNil())
	Expect(stsSize).NotTo(BeNil())
	Expect(mdbSize.Cmp(*stsSize)).To(Equal(0))

	By("Expecting PVCs to have been resized")
	pvcList := corev1.PersistentVolumeClaimList{}
	listOpts := client.ListOptions{
		LabelSelector: klabels.SelectorFromSet(
			labels.NewLabelsBuilder().
				WithMariaDBSelectorLabels(mdb).
				WithPVCRole(builder.StorageVolumeRole).
				Build(),
		),
		Namespace: mdb.GetNamespace(),
	}
	Expect(k8sClient.List(testCtx, &pvcList, &listOpts)).To(Succeed())
	for _, p := range pvcList.Items {
		pvcSize := p.Spec.Resources.Requests[corev1.ResourceStorage]
		Expect(mdbSize.Cmp(pvcSize)).To(Equal(0))
	}
}

func testMaxscale(mdb *mariadbv1alpha1.MariaDB, mxs *mariadbv1alpha1.MaxScale) {
	mdbKey := client.ObjectKeyFromObject(mdb)
	mxsKey := client.ObjectKeyFromObject(mxs)

	applyMaxscaleTestConfig(mxs)

	By("Creating MaxScale")
	Expect(k8sClient.Create(testCtx, mxs)).To(Succeed())
	DeferCleanup(func() {
		deleteMaxScale(mxsKey, true)
	})

	By("Point MariaDB to MaxScale")
	Eventually(func(g Gomega) bool {
		if err := k8sClient.Get(testCtx, mdbKey, mdb); err != nil {
			return false
		}
		mdb.Spec.MaxScaleRef = &corev1.ObjectReference{
			Name:      mxsKey.Name,
			Namespace: mxsKey.Namespace,
		}
		g.Expect(k8sClient.Update(testCtx, mdb)).To(Succeed())
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	By("Point MaxScale to MariaDB")
	Eventually(func(g Gomega) bool {
		if err := k8sClient.Get(testCtx, mxsKey, mxs); err != nil {
			return false
		}
		mxs.Spec.MariaDBRef = &mariadbv1alpha1.MariaDBRef{
			ObjectReference: corev1.ObjectReference{
				Name:      mdbKey.Name,
				Namespace: mdbKey.Namespace,
			},
		}
		g.Expect(k8sClient.Update(testCtx, mxs)).To(Succeed())
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting MariaDB to be ready eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, mdbKey, mdb); err != nil {
			return false
		}
		return mdb.IsReady()
	}, testHighTimeout, testInterval).Should(BeTrue())

	By("Expecting MaxScale to be ready eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, mxsKey, mxs); err != nil {
			return false
		}
		return mxs.IsReady()
	}, testHighTimeout, testInterval).Should(BeTrue())

	By("Expecting servers to be ready eventually")
	Eventually(func(g Gomega) bool {
		if err := k8sClient.Get(testCtx, mxsKey, mxs); err != nil {
			return false
		}
		for _, srv := range mxs.Status.Servers {
			g.Expect(srv.IsReady()).To(BeTrue())
		}
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting monitor to be running eventually")
	Eventually(func(g Gomega) bool {
		if err := k8sClient.Get(testCtx, mxsKey, mxs); err != nil {
			return false
		}
		g.Expect(ptr.Deref(
			mxs.Status.Monitor,
			mariadbv1alpha1.MaxScaleResourceStatus{},
		).State).To(Equal("Running"))
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting services to be started eventually")
	Eventually(func(g Gomega) bool {
		if err := k8sClient.Get(testCtx, mxsKey, mxs); err != nil {
			return false
		}
		for _, svc := range mxs.Status.Services {
			g.Expect(svc.State).To(Equal("Started"))
		}
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting listeners to be running")
	Eventually(func(g Gomega) bool {
		if err := k8sClient.Get(testCtx, mxsKey, mxs); err != nil {
			return false
		}
		for _, listener := range mxs.Status.Listeners {
			g.Expect(listener.State).To(Equal("Running"))
		}
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting primary to be set eventually")
	Eventually(func(g Gomega) bool {
		if err := k8sClient.Get(testCtx, mdbKey, mdb); err != nil {
			return false
		}
		if err := k8sClient.Get(testCtx, mxsKey, mxs); err != nil {
			return false
		}
		g.Expect(mdb.Status.CurrentPrimary).ToNot(BeNil())
		g.Expect(mdb.Status.CurrentPrimaryPodIndex).ToNot(BeNil())
		g.Expect(mxs.Status.PrimaryServer).NotTo(BeNil())
		return true
	}, testHighTimeout, testInterval).Should(BeTrue())

	By("Expecting to create a ServiceAccount")
	var svcAcc corev1.ServiceAccount
	Expect(k8sClient.Get(testCtx, mxsKey, &svcAcc)).To(Succeed())

	By("Expecting to create a StatefulSet")
	var sts appsv1.StatefulSet
	Expect(k8sClient.Get(testCtx, mxsKey, &sts)).To(Succeed())

	By("Expecting to create a Service")
	var svc corev1.Service
	Expect(k8sClient.Get(testCtx, mxsKey, &svc)).To(Succeed())

	By("Expecting to create a GUI Service")
	var guiSvc corev1.Service
	Expect(k8sClient.Get(testCtx, mxs.GuiServiceKey(), &guiSvc)).To(Succeed())

	By("Expecting Connection to be ready eventually")
	Eventually(func() bool {
		var conn mariadbv1alpha1.Connection
		if err := k8sClient.Get(testCtx, mxs.ConnectionKey(), &conn); err != nil {
			return false
		}
		return conn.IsReady()
	}, testTimeout, testInterval).Should(BeTrue())

	type secretRef struct {
		name        string
		keySelector corev1.SecretKeySelector
	}
	secretKeyRefs := []secretRef{
		{
			name:        "admin",
			keySelector: mxs.Spec.Auth.AdminPasswordSecretKeyRef.SecretKeySelector,
		},
		{
			name:        "client",
			keySelector: mxs.Spec.Auth.ClientPasswordSecretKeyRef.SecretKeySelector,
		},
		{
			name:        "server",
			keySelector: mxs.Spec.Auth.ServerPasswordSecretKeyRef.SecretKeySelector,
		},
		{
			name:        "monitor",
			keySelector: mxs.Spec.Auth.MonitorPasswordSecretKeyRef.SecretKeySelector,
		},
	}
	if mxs.IsHAEnabled() {
		secretKeyRefs = append(secretKeyRefs, secretRef{
			name:        "sync",
			keySelector: mxs.Spec.Auth.SyncPasswordSecretKeyRef.SecretKeySelector,
		})
	}
	if mxs.AreMetricsEnabled() {
		secretKeyRefs = append(secretKeyRefs, secretRef{
			name:        "metrics",
			keySelector: mxs.Spec.Auth.MetricsPasswordSecretKeyRef.SecretKeySelector,
		})
	}

	for _, secretKeyRef := range secretKeyRefs {
		By(fmt.Sprintf("Expecting to create a '%s' Secret eventually", secretKeyRef.name))
		key := types.NamespacedName{
			Name:      secretKeyRef.keySelector.Name,
			Namespace: mxs.Namespace,
		}
		expectSecretToExist(testCtx, k8sClient, key, secretKeyRef.keySelector.Key)
	}

	if mxs.AreMetricsEnabled() {
		By("Expecting to create a exporter Deployment eventually")
		Eventually(func(g Gomega) bool {
			var deploy appsv1.Deployment
			if err := k8sClient.Get(testCtx, mxs.MetricsKey(), &deploy); err != nil {
				return false
			}
			expectedImage := os.Getenv("RELATED_IMAGE_EXPORTER_MAXSCALE")
			g.Expect(expectedImage).ToNot(BeEmpty())

			By("Expecting Deployment to have exporter image")
			g.Expect(deploy.Spec.Template.Spec.Containers).To(ContainElement(MatchFields(IgnoreExtras,
				Fields{
					"Image": Equal(expectedImage),
				})))

			By("Expecting Deployment to be ready")
			return deploymentReady(&deploy)
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a ServiceMonitor eventually")
		Eventually(func(g Gomega) bool {
			var svcMonitor monitoringv1.ServiceMonitor
			if err := k8sClient.Get(testCtx, mxs.MetricsKey(), &svcMonitor); err != nil {
				return false
			}
			g.Expect(svcMonitor.Spec.Selector.MatchLabels).NotTo(BeEmpty())
			g.Expect(svcMonitor.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app.kubernetes.io/name", "exporter"))
			g.Expect(svcMonitor.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app.kubernetes.io/instance", mxs.MetricsKey().Name))
			g.Expect(svcMonitor.Spec.Endpoints).To(HaveLen(int(mxs.Spec.Replicas)))
			return true
		}, testTimeout, testInterval).Should(BeTrue())
	}
}

func testValidCredentials(username string, passwordSecretKeyRef corev1.SecretKeySelector) {
	key := types.NamespacedName{
		Name:      fmt.Sprintf("test-creds-conn-%s", uuid.New().String()),
		Namespace: testNamespace,
	}

	conn := mariadbv1alpha1.Connection{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: mariadbv1alpha1.ConnectionSpec{
			ConnectionTemplate: mariadbv1alpha1.ConnectionTemplate{
				SecretName: ptr.To(key.Name),
			},
			MariaDBRef: &mariadbv1alpha1.MariaDBRef{
				ObjectReference: corev1.ObjectReference{
					Name: testMdbkey.Name,
				},
				WaitForIt: true,
			},
			Username:             username,
			PasswordSecretKeyRef: passwordSecretKeyRef,
			Database:             &testDatabase,
		},
	}
	By("Creating Connection")
	Expect(k8sClient.Create(testCtx, &conn)).To(Succeed())
	DeferCleanup(func() {
		Expect(k8sClient.Delete(testCtx, &conn)).To(Succeed())
	})

	By("Expecting Connection to be ready eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, &conn); err != nil {
			return false
		}
		return conn.IsReady()
	}, testTimeout, testInterval).Should(BeTrue())
}

func applyMariadbTestConfig(mdb *mariadbv1alpha1.MariaDB) *mariadbv1alpha1.MariaDB {
	mdb.Spec.ContainerTemplate.ReadinessProbe = &corev1.Probe{
		InitialDelaySeconds: 10,
	}
	mdb.Spec.ContainerTemplate.LivenessProbe = &corev1.Probe{
		InitialDelaySeconds: 30,
	}
	mdb.Spec.Resources = &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("300m"),
			"memory": resource.MustParse("256Mi"),
		},
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse("300m"),
			"memory": resource.MustParse("256Mi"),
		},
	}
	return mdb
}

func applyMaxscaleTestConfig(mxs *mariadbv1alpha1.MaxScale) *mariadbv1alpha1.MaxScale {
	mxs.Spec.ContainerTemplate.ReadinessProbe = &corev1.Probe{
		InitialDelaySeconds: 10,
	}
	mxs.Spec.ContainerTemplate.LivenessProbe = &corev1.Probe{
		InitialDelaySeconds: 30,
	}
	mxs.Spec.Resources = &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("200m"),
			"memory": resource.MustParse("128Mi"),
		},
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse("200m"),
			"memory": resource.MustParse("128Mi"),
		},
	}
	return mxs
}

func getS3WithBucket(bucket, prefix string) *mariadbv1alpha1.S3 {
	return &mariadbv1alpha1.S3{
		Bucket:   bucket,
		Prefix:   prefix,
		Endpoint: "minio.minio.svc.cluster.local:9000",
		Region:   "us-east-1",
		AccessKeyIdSecretKeyRef: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: "minio",
			},
			Key: "access-key-id",
		},
		SecretAccessKeySecretKeyRef: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: "minio",
			},
			Key: "secret-access-key",
		},
		TLS: &mariadbv1alpha1.TLS{
			Enabled: true,
			CASecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "minio-ca",
				},
				Key: "ca.crt",
			},
		},
	}
}

func getBackupWithStorage(key types.NamespacedName, storage mariadbv1alpha1.BackupStorage) *mariadbv1alpha1.Backup {
	return &mariadbv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: mariadbv1alpha1.BackupSpec{
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: corev1.ObjectReference{
					Name: testMdbkey.Name,
				},
				WaitForIt: true,
			},
			InheritMetadata: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"k8s.mariadb.com/test": "test",
				},
				Annotations: map[string]string{
					"k8s.mariadb.com/test": "test",
				},
			},
			Storage: storage,
		},
	}
}

func getBackupWithPVCStorage(key types.NamespacedName) *mariadbv1alpha1.Backup {
	return getBackupWithStorage(key, mariadbv1alpha1.BackupStorage{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("100Mi"),
				},
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
		},
	})
}

func getBackupWithS3Storage(key types.NamespacedName, bucket, prefix string) *mariadbv1alpha1.Backup {
	return getBackupWithStorage(key, mariadbv1alpha1.BackupStorage{
		S3: getS3WithBucket(bucket, prefix),
	})
}

func getBackupWithVolumeStorage(key types.NamespacedName) *mariadbv1alpha1.Backup {
	return getBackupWithStorage(key, mariadbv1alpha1.BackupStorage{
		Volume: &corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
}

func expectMariadbReady(ctx context.Context, k8sClient client.Client, key types.NamespacedName) {
	By("Expecting MariaDB to be ready eventually")
	expectMariadbFn(ctx, k8sClient, key, func(mdb *mariadbv1alpha1.MariaDB) bool {
		return mdb.IsReady()
	})
}

func expectMariadbFn(ctx context.Context, k8sClient client.Client, key types.NamespacedName, fn func(mdb *mariadbv1alpha1.MariaDB) bool) {
	var mdb mariadbv1alpha1.MariaDB
	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(ctx, key, &mdb)).To(Succeed())
		return fn(&mdb)
	}, testHighTimeout, testInterval).Should(BeTrue())
}

func expectSecretToExist(ctx context.Context, k8sClient client.Client, key types.NamespacedName, secretKey string) {
	Eventually(func(g Gomega) bool {
		var secret corev1.Secret
		key := types.NamespacedName{
			Name:      key.Name,
			Namespace: key.Namespace,
		}
		if err := k8sClient.Get(ctx, key, &secret); err != nil {
			return false
		}
		Expect(secret.Data[secretKey]).ToNot(BeEmpty())
		return true
	}, testTimeout, testInterval).Should(BeTrue())
}

func deploymentReady(deploy *appsv1.Deployment) bool {
	for _, c := range deploy.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func deleteMariaDB(mdb *mariadbv1alpha1.MariaDB) {
	Expect(k8sClient.Delete(testCtx, mdb)).To(Succeed())

	Eventually(func(g Gomega) bool {
		listOpts := &client.ListOptions{
			LabelSelector: klabels.SelectorFromSet(
				labels.NewLabelsBuilder().
					WithMariaDBSelectorLabels(mdb).
					Build(),
			),
			Namespace: mdb.GetNamespace(),
		}
		pvcList := &corev1.PersistentVolumeClaimList{}
		g.Expect(k8sClient.List(testCtx, pvcList, listOpts)).To(Succeed())

		for _, pvc := range pvcList.Items {
			g.Expect(k8sClient.Delete(testCtx, &pvc)).To(Succeed())
		}
		return true
	}, 30*time.Second, 1*time.Second).Should(BeTrue())
}

func deleteMaxScale(key types.NamespacedName, assertPVCDeletion bool) {
	mxs := mariadbv1alpha1.MaxScale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	err := k8sClient.Delete(testCtx, &mxs)
	if err != nil && !apierrors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred())
	}

	if !assertPVCDeletion {
		return
	}
	Eventually(func(g Gomega) bool {
		listOpts := &client.ListOptions{
			LabelSelector: klabels.SelectorFromSet(
				labels.NewLabelsBuilder().
					WithMaxScaleSelectorLabels(&mxs).
					Build(),
			),
			Namespace: mxs.GetNamespace(),
		}
		pvcList := &corev1.PersistentVolumeClaimList{}
		err := k8sClient.List(testCtx, pvcList, listOpts)
		if err != nil && !apierrors.IsNotFound(err) {
			g.Expect(err).ToNot(HaveOccurred())
		}
		return len(pvcList.Items) == 0
	}, testHighTimeout, testInterval).Should(BeTrue())
}
