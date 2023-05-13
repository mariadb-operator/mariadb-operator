/*
Copyright 2022.

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

package controllers

import (
	"context"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/builder"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

const (
	testTimeout  = time.Second * 60
	testInterval = time.Second * 1
)

var (
	testNamespace        = "default"
	testStorageClassName = "standard"
	testMariaDbName      = "mariadb-test"

	testUser           = "test"
	testPwdSecretKey   = "passsword"
	testPwdSecretName  = "password-test"
	testDatabase       = "test"
	testConnSecretName = "test-conn"
	testConnSecretKey  = "dsn"
)

var testMariaDbKey types.NamespacedName
var testMariaDb mariadbv1alpha1.MariaDB
var testPwdKey types.NamespacedName
var testPwd v1.Secret

func createTestData(ctx context.Context, k8sClient client.Client) {
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
			Image: mariadbv1alpha1.Image{
				Repository: "mariadb",
				Tag:        "10.7.4",
			},
			VolumeClaimTemplate: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &testStorageClassName,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": resource.MustParse("100Mi"),
					},
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
			},
			MyCnf: func() *string {
				cfg := `[mysqld]
				bind-address=0.0.0.0
				default_storage_engine=InnoDB
				binlog_format=row
				innodb_autoinc_lock_mode=2
				max_allowed_packet=256M`
				return &cfg
			}(),
			PodSecurityContext: &corev1.PodSecurityContext{
				RunAsUser: func() *int64 { u := int64(0); return &u }(),
			},
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: func() *bool { b := false; return &b }(),
			},
			LivenessProbe: &v1.Probe{
				ProbeHandler: v1.ProbeHandler{
					Exec: &v1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mysql -u root -p${MARIADB_ROOT_PASSWORD} -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      5,
				PeriodSeconds:       5,
			},
			ReadinessProbe: &v1.Probe{
				ProbeHandler: v1.ProbeHandler{
					Exec: &v1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mysql -u root -p${MARIADB_ROOT_PASSWORD} -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      5,
				PeriodSeconds:       5,
			},
			Service: &mariadbv1alpha1.Service{
				Type: corev1.ServiceTypeLoadBalancer,
				Annotations: map[string]string{
					"metallb.universe.tf/loadBalancerIPs": "172.18.0.100",
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
