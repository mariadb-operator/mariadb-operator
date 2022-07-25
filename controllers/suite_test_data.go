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
	"github.com/mmontes11/mariadb-operator/pkg/builders"
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
	timeout  = time.Second * 30
	interval = time.Second * 1
)

var (
	defaultNamespace         = "default"
	defaultStorageClass      = "standard"
	mariaDbName              = "mariadb-test"
	mariaDbRootPwdSecretName = "root-test"
	mariaDbRootPwdSecretKey  = "passsword"
)

var mariaDbKey types.NamespacedName
var mariaDb databasev1alpha1.MariaDB
var mariaDbRootPwdKey types.NamespacedName
var mariaDbRootPwd v1.Secret
var storageSize resource.Quantity

func createTestData(ctx context.Context, k8sClient client.Client) {
	By("Creating initial test data")

	mariaDbRootPwdKey = types.NamespacedName{
		Name:      mariaDbRootPwdSecretName,
		Namespace: defaultNamespace,
	}
	mariaDbRootPwd = v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariaDbRootPwdKey.Name,
			Namespace: mariaDbRootPwdKey.Namespace,
		},
		Data: map[string][]byte{
			mariaDbRootPwdSecretKey: []byte("mariadb"),
		},
	}
	Expect(k8sClient.Create(ctx, &mariaDbRootPwd)).To(Succeed())

	mariaDbKey = types.NamespacedName{
		Name:      mariaDbName,
		Namespace: defaultNamespace,
	}
	storageSize = resource.MustParse("100Mi")
	mariaDb = databasev1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariaDbKey.Name,
			Namespace: mariaDbKey.Namespace,
		},
		Spec: databasev1alpha1.MariaDBSpec{
			RootPasswordSecretKeyRef: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: mariaDbRootPwdKey.Name,
				},
				Key: mariaDbRootPwdSecretKey,
			},
			Image: databasev1alpha1.Image{
				Repository: "mariadb",
				Tag:        "10.7.4",
			},
			Storage: databasev1alpha1.Storage{
				ClassName: defaultStorageClass,
				Size:      storageSize,
			},
		},
	}
	Expect(k8sClient.Create(ctx, &mariaDb)).To(Succeed())

	By("Expecting MariaDB to be ready eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, mariaDbKey, &mariaDb); err != nil {
			return false
		}
		return mariaDb.IsReady()
	}, timeout, interval).Should(BeTrue())
}

func deleteTestData(ctx context.Context, k8sClient client.Client) {
	By("Deleting initial test data")

	Expect(k8sClient.Delete(ctx, &mariaDb)).To(Succeed())
	Expect(k8sClient.Delete(ctx, &mariaDbRootPwd)).To(Succeed())

	var pvc corev1.PersistentVolumeClaim
	Expect(k8sClient.Get(ctx, builders.GetPVCKey(&mariaDb), &pvc)).To(Succeed())
	Expect(k8sClient.Delete(ctx, &pvc)).To(Succeed())
}
