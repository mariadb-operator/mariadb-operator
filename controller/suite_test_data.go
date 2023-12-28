package controller

import (
	"context"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/pkg/docker"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

var (
	testVeryHighTimeout = 5 * time.Minute
	testHighTimeout     = 3 * time.Minute
	testTimeout         = 1 * time.Minute
	testInterval        = 1 * time.Second

	testNamespace      = "default"
	testMariaDbName    = "mariadb-test"
	testMariaDbKey     types.NamespacedName
	testMariaDb        mariadbv1alpha1.MariaDB
	testPwdKey         types.NamespacedName
	testPwd            v1.Secret
	testUser           = "test"
	testPwdSecretKey   = "passsword"
	testPwdSecretName  = "password-test"
	testDatabase       = "test"
	testConnSecretName = "test-conn"
	testConnSecretKey  = "dsn"
	testCASecretKey    = types.NamespacedName{
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

func createTestData(ctx context.Context, k8sClient client.Client, env environment.Environment) {
	var testCidrPrefix, err = docker.GetKindCidrPrefix()
	Expect(testCidrPrefix, err).ShouldNot(Equal(""))

	testPwdKey = types.NamespacedName{
		Name:      testPwdSecretName,
		Namespace: testNamespace,
	}
	testPwd = v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPwdKey.Name,
			Namespace: testPwdKey.Namespace,
		},
		Data: map[string][]byte{
			testPwdSecretKey: []byte("test"),
		},
	}
	Expect(k8sClient.Create(ctx, &testPwd)).To(Succeed())

	testMariaDbKey = types.NamespacedName{
		Name:      testMariaDbName,
		Namespace: testNamespace,
	}
	testMariaDb = mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testMariaDbKey.Name,
			Namespace: testMariaDbKey.Namespace,
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: func() *bool { b := false; return &b }(),
				},
			},
			PodTemplate: mariadbv1alpha1.PodTemplate{
				PodSecurityContext: &corev1.PodSecurityContext{
					RunAsUser: func() *int64 { u := int64(0); return &u }(),
				},
			},
			Image:           env.RelatedMariadbImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			InheritMetadata: &mariadbv1alpha1.InheritMetadata{
				Labels: map[string]string{
					"mariadb.mmontes.io/test": "test",
				},
				Annotations: map[string]string{
					"mariadb.mmontes.io/test": "test",
				},
			},
			RootPasswordSecretKeyRef: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: testPwdKey.Name,
				},
				Key: testPwdSecretKey,
			},
			Username: &testUser,
			PasswordSecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: testPwdKey.Name,
				},
				Key: testPwdSecretKey,
			},
			Database: &testDatabase,
			Connection: &mariadbv1alpha1.ConnectionTemplate{
				SecretName: &testConnSecretName,
				SecretTemplate: &mariadbv1alpha1.SecretTemplate{
					Key: &testConnSecretKey,
				},
			},
			VolumeClaimTemplate: mariadbv1alpha1.VolumeClaimTemplate{
				PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"storage": resource.MustParse("100Mi"),
						},
					},
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
				},
			},
			MyCnf: func() *string {
				cfg := `[mariadb]
				bind-address=*
				default_storage_engine=InnoDB
				binlog_format=row
				innodb_autoinc_lock_mode=2
				max_allowed_packet=256M`
				return &cfg
			}(),
			Port: 3306,
			Service: &mariadbv1alpha1.ServiceTemplate{
				Type: corev1.ServiceTypeLoadBalancer,
				Annotations: map[string]string{
					"metallb.universe.tf/loadBalancerIPs": testCidrPrefix + ".0.100",
				},
			},
			Metrics: &mariadbv1alpha1.Metrics{
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
				PasswordSecretKeyRef: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: testPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, &testMariaDb)).To(Succeed())

	By("Expecting MariaDB to be ready eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, testMariaDbKey, &testMariaDb); err != nil {
			return false
		}
		return testMariaDb.IsReady()
	}, testTimeout, testInterval).Should(BeTrue())
}

func deleteTestData(ctx context.Context, k8sClient client.Client) {
	Expect(k8sClient.Delete(ctx, &testMariaDb)).To(Succeed())
	Expect(k8sClient.Delete(ctx, &testPwd)).To(Succeed())

	var pvc corev1.PersistentVolumeClaim
	Expect(k8sClient.Get(ctx, builder.PVCKey(&testMariaDb), &pvc)).To(Succeed())
	Expect(k8sClient.Delete(ctx, &pvc)).To(Succeed())
}

func testS3WithBucket(bucket string) *mariadbv1alpha1.S3 {
	return &mariadbv1alpha1.S3{
		Bucket:   bucket,
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
					Name: testMariaDbName,
				},
				WaitForIt: true,
			},
			Storage: storage,
		},
	}
}

func testBackupWithPVCStorage(key types.NamespacedName) *mariadbv1alpha1.Backup {
	return testBackupWithStorage(key, mariadbv1alpha1.BackupStorage{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
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

func testBackupWithS3Storage(key types.NamespacedName, bucket string) *mariadbv1alpha1.Backup {
	return testBackupWithStorage(key, mariadbv1alpha1.BackupStorage{
		S3: testS3WithBucket(bucket),
	})
}

func testBackupWithVolumeStorage(key types.NamespacedName) *mariadbv1alpha1.Backup {
	return testBackupWithStorage(key, mariadbv1alpha1.BackupStorage{
		Volume: &corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
}
