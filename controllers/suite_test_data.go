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

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/builder"
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
	testTimeout  = time.Second * 30
	testInterval = time.Second * 1
)

var (
	testNamespace         = "default"
	testStorageClassName  = "standard"
	testMariaDbName       = "mariadb-test"
	testRootPwdSecretName = "root-test"
	testRootPwdSecretKey  = "passsword"
)

var testMariaDbKey types.NamespacedName
var testMariaDb databasev1alpha1.MariaDB
var testRootPwdKey types.NamespacedName
var testRootPwd v1.Secret

func createTestData(ctx context.Context, k8sClient client.Client) {
	testRootPwdKey = types.NamespacedName{
		Name:      testRootPwdSecretName,
		Namespace: testNamespace,
	}
	testRootPwd = v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testRootPwdKey.Name,
			Namespace: testRootPwdKey.Namespace,
		},
		Data: map[string][]byte{
			testRootPwdSecretKey: []byte("mariadb"),
		},
	}
	Expect(k8sClient.Create(ctx, &testRootPwd)).To(Succeed())

	testMariaDbKey = types.NamespacedName{
		Name:      testMariaDbName,
		Namespace: testNamespace,
	}
	testMariaDb = databasev1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testMariaDbKey.Name,
			Namespace: testMariaDbKey.Namespace,
		},
		Spec: databasev1alpha1.MariaDBSpec{
			RootPasswordSecretKeyRef: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: testRootPwdKey.Name,
				},
				Key: testRootPwdSecretKey,
			},
			Image: databasev1alpha1.Image{
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
	Expect(k8sClient.Delete(ctx, &testRootPwd)).To(Succeed())

	var pvc corev1.PersistentVolumeClaim
	Expect(k8sClient.Get(ctx, builder.GetPVCKey(&testMariaDb), &pvc)).To(Succeed())
	Expect(k8sClient.Delete(ctx, &pvc)).To(Succeed())
}
