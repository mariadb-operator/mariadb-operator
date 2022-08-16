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

package v1alpha1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MariaDB webhook", func() {
	Context("When updating a MariaDB", func() {
		It("Should validate", func() {
			By("Creating MariaDB")
			key := types.NamespacedName{
				Name:      "mariadb-webhook",
				Namespace: testNamespace,
			}
			test := "test"
			mariaDb := MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: MariaDBSpec{
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secret",
						},
						Key: "root-password",
					},
					Database: &test,
					Username: &test,
					PasswordSecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secret",
						},
						Key: "password",
					},
					Image: Image{
						Repository: "mariadb",
						Tag:        "10.7.4",
					},
					Port: 3306,
					Storage: Storage{
						ClassName: "standard",
						Size:      resource.MustParse("100Mi"),
					},
					BootstrapFromBackup: &BootstrapFromBackup{
						BackupRef: corev1.LocalObjectReference{
							Name: "backup",
						},
					},
					Metrics: &Metrics{
						Exporter: Exporter{
							Image: Image{
								Repository: "prom/mysqld-exporter",
								Tag:        "v0.14.0",
							},
						},
						ServiceMonitor: ServiceMonitor{
							PrometheusRelease: "prometheus",
						},
					},
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("100m"),
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "TZ",
							Value: "SYSTEM",
						},
					},
					EnvFrom: []corev1.EnvFromSource{
						{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mariadb",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &mariaDb)).To(Succeed())

			// TODO: migrate to Ginkgo v2 and use Ginkgo table tests
			// https://github.com/kubernetes-sigs/kubebuilder/issues/2532
			// https://onsi.github.io/ginkgo/#table-specs-patterns
			tt := []struct {
				by      string
				patchFn func(mdb *MariaDB)
				wantErr bool
			}{
				{
					by: "Updating RootPasswordSecretKeyRef",
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.RootPasswordSecretKeyRef.Key = "another-password"
					},
					wantErr: true,
				},
				{
					by: "Updating Database",
					patchFn: func(mdb *MariaDB) {
						another := "another-database"
						mdb.Spec.Database = &another
					},
					wantErr: true,
				},
				{
					by: "Updating Username",
					patchFn: func(mdb *MariaDB) {
						another := "another-username"
						mdb.Spec.Username = &another
					},
					wantErr: true,
				},
				{
					by: "Updating PasswordSecretKeyRef",
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.PasswordSecretKeyRef.Key = "another-password"
					},
					wantErr: true,
				},
				{
					by: "Updating Image",
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.Image.Tag = "10.7.5"
					},
					wantErr: true,
				},
				{
					by: "Updating Port",
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.Port = 3307
					},
					wantErr: false,
				},
				{
					by: "Updating Storage",
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.Storage.ClassName = "fast-storage"
					},
					wantErr: true,
				},
				{
					by: "Updating BootstrapFromBackup",
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.BootstrapFromBackup.BackupRef.Name = "another-backup"
					},
					wantErr: true,
				},
				{
					by: "Updating Metrics",
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.Metrics.Exporter.Image.Tag = "v0.14.1"
					},
					wantErr: false,
				},
				{
					by: "Updating Resources",
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.Resources = &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("200m"),
							},
						}
					},
					wantErr: false,
				},
				{
					by: "Updating Env",
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.Env = []corev1.EnvVar{
							{
								Name:  "FOO",
								Value: "foo",
							},
						}
					},
					wantErr: false,
				},
				{
					by: "Updating EnvFrom",
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.EnvFrom = []corev1.EnvFromSource{
							{
								ConfigMapRef: &corev1.ConfigMapEnvSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "mariadb",
									},
								},
							},
						}
					},
					wantErr: false,
				},
			}

			for _, t := range tt {
				By(t.by)
				Expect(k8sClient.Get(testCtx, key, &mariaDb)).To(Succeed())

				patch := client.MergeFrom(mariaDb.DeepCopy())
				t.patchFn(&mariaDb)

				err := k8sClient.Patch(testCtx, &mariaDb, patch)
				if t.wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})
})
