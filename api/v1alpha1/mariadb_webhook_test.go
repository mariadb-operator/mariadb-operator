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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MariaDB webhook", func() {
	Context("When creating a MariaDB", func() {
		It("Should validate", func() {
			meta := metav1.ObjectMeta{
				Name:      "mariadb-create-webhook",
				Namespace: testNamespace,
			}
			// TODO: migrate to Ginkgo v2 and use Ginkgo table tests
			// https://github.com/mariadb-operator/mariadb-operator/issues/3
			tt := []struct {
				by      string
				mdb     MariaDB
				wantErr bool
			}{
				{
					by: "no source",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							BootstrapFrom: nil,
						},
					},
					wantErr: false,
				},
				{
					by: "valid source",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							BootstrapFrom: &RestoreSource{
								BackupRef: &corev1.LocalObjectReference{
									Name: "backup-webhook",
								},
							},
						},
					},
					wantErr: false,
				},
				{
					by: "invalid source",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							BootstrapFrom: &RestoreSource{
								Physical: func() *bool {
									p := true
									return &p
								}(),
							},
						},
					},
					wantErr: true,
				},
				{
					by: "valid replication",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Replication: &Replication{
								PrimaryPodIndex: 1,
							},
							Replicas: 3,
						},
					},
					wantErr: false,
				},
				{
					by: "valid replication with wait point and retries",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Replication: &Replication{
								WaitPoint: func() *WaitPoint { w := WaitPointAfterCommit; return &w }(),
								Timeout:   &metav1.Duration{Duration: time.Duration(1 * time.Second)},
								Retries:   func() *int { r := 3; return &r }(),
							},
							Replicas: 3,
						},
					},
					wantErr: false,
				},
				{
					by: "invalid replication replicas",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Replication: &Replication{
								WaitPoint: func() *WaitPoint { w := WaitPointAfterCommit; return &w }(),
								Timeout:   &metav1.Duration{Duration: time.Duration(1 * time.Second)},
								Retries:   func() *int { r := 3; return &r }(),
							},
							Replicas: 1,
						},
					},
					wantErr: true,
				},
				{
					by: "invalid primary pod index",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Replication: &Replication{
								WaitPoint:       func() *WaitPoint { w := WaitPointAfterCommit; return &w }(),
								Timeout:         &metav1.Duration{Duration: time.Duration(1 * time.Second)},
								Retries:         func() *int { r := 3; return &r }(),
								PrimaryPodIndex: 4,
							},
							Replicas: 3,
						},
					},
					wantErr: true,
				},
				{
					by: "invalid gtid",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Replication: &Replication{
								Gtid: Gtid("foo"),
							},
							Replicas: 3,
						},
					},
					wantErr: true,
				},
				{
					by: "invalid wait point",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Replication: &Replication{
								WaitPoint: func() *WaitPoint { w := WaitPoint("foo"); return &w }(),
							},
							Replicas: 3,
						},
					},
					wantErr: true,
				},
				{
					by: "invalid primary pod index",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Replication: &Replication{
								PrimaryPodIndex: 4,
							},
							Replicas: 3,
						},
					},
					wantErr: true,
				},
				{
					by: "invalid replicas",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Replicas: 3,
						},
					},
					wantErr: true,
				},
				{
					by: "invalid pod disruption budget",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							PodDisruptionBudget: &PodDisruptionBudget{
								MaxUnavailable: func() *intstr.IntOrString { i := intstr.FromString("50%"); return &i }(),
								MinAvailable:   func() *intstr.IntOrString { i := intstr.FromString("50%"); return &i }(),
							},
						},
					},
					wantErr: true,
				},
				{
					by: "valid pod disruption budget",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							PodDisruptionBudget: &PodDisruptionBudget{
								MaxUnavailable: func() *intstr.IntOrString { i := intstr.FromString("50%"); return &i }(),
							},
						},
					},
					wantErr: false,
				},
			}

			for _, t := range tt {
				By(t.by)
				_ = k8sClient.Delete(testCtx, &t.mdb)
				err := k8sClient.Create(testCtx, &t.mdb)
				if t.wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})

	Context("When updating a MariaDB", func() {
		It("Should validate", func() {
			By("Creating MariaDB")
			key := types.NamespacedName{
				Name:      "mariadb-update-webhook",
				Namespace: testNamespace,
			}
			test := "test"
			storageClassName := "standard"
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
					VolumeClaimTemplate: corev1.PersistentVolumeClaimSpec{
						StorageClassName: &storageClassName,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"storage": resource.MustParse("100Mi"),
							},
						},
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
					},
					MyCnf: func() *string { c := "foo"; return &c }(),
					BootstrapFrom: &RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
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
			// https://github.com/mariadb-operator/mariadb-operator/issues/3
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
						newClass := "fast-storage"
						mdb.Spec.VolumeClaimTemplate.StorageClassName = &newClass
					},
					wantErr: true,
				},
				{
					by: "Updating MyCnf",
					patchFn: func(mdb *MariaDB) {
						newCnf := "bar"
						mdb.Spec.MyCnf = &newCnf
					},
					wantErr: true,
				},
				{
					by: "Updating MyCnfConfigMapKeyRef",
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.MyCnfConfigMapKeyRef = &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "my-cnf-configmap",
							},
							Key: "config",
						}
					},
					wantErr: false,
				},
				{
					by: "Updating BootstrapFromBackup",
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.BootstrapFrom.BackupRef.Name = "another-backup"
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
