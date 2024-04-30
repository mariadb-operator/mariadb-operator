package controller

import (
	"context"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/docker"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

var (
	testVeryHighTimeout = 4 * time.Minute
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
	testPwdSecretKey = "passsword"
	testUser         = "test"
	testDatabase     = "test"
	testConnKey      = types.NamespacedName{
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

func expectMariadbReady(ctx context.Context, k8sClient client.Client) {
	var mdb mariadbv1alpha1.MariaDB
	By("Expecting MariaDB to be ready eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, testMdbkey, &mdb); err != nil {
			return false
		}
		return mdb.IsReady()
	}, testHighTimeout, testInterval).Should(BeTrue())
}

func createTestData(ctx context.Context, k8sClient client.Client, env environment.OperatorEnv) {
	var testCidrPrefix, err = docker.GetKindCidrPrefix()
	Expect(testCidrPrefix).ShouldNot(Equal(""))
	Expect(err).ToNot(HaveOccurred())

	password := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPwdKey.Name,
			Namespace: testPwdKey.Namespace,
		},
		Data: map[string][]byte{
			testPwdSecretKey: []byte("MariaDB11!"),
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
						Key: testPwdSecretKey,
					},
				},
			},
			Storage: mariadbv1alpha1.Storage{
				Size: ptr.To(resource.MustParse("300Mi")),
			},
		},
	}
	Expect(k8sClient.Create(ctx, &mdb)).To(Succeed())
	expectMariadbReady(ctx, k8sClient)
}

func testS3WithBucket(bucket, prefix string) *mariadbv1alpha1.S3 {
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

func testBackupWithStorage(key types.NamespacedName, storage mariadbv1alpha1.BackupStorage) *mariadbv1alpha1.Backup {
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

func testBackupWithPVCStorage(key types.NamespacedName) *mariadbv1alpha1.Backup {
	return testBackupWithStorage(key, mariadbv1alpha1.BackupStorage{
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

func testBackupWithS3Storage(key types.NamespacedName, bucket, prefix string) *mariadbv1alpha1.Backup {
	return testBackupWithStorage(key, mariadbv1alpha1.BackupStorage{
		S3: testS3WithBucket(bucket, prefix),
	})
}

func testBackupWithVolumeStorage(key types.NamespacedName) *mariadbv1alpha1.Backup {
	return testBackupWithStorage(key, mariadbv1alpha1.BackupStorage{
		Volume: &corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
}
