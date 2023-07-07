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
								FileName: func() *string { b := "backup.sql"; return &b }(),
							},
						},
					},
					wantErr: true,
				},
				{
					by: "valid galera",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Galera: &Galera{
								Enabled:        true,
								SST:            "mariabackup",
								ReplicaThreads: 1,
							},
							Replicas: 3,
						},
					},
					wantErr: false,
				},
				{
					by: "valid replication",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Replication: &Replication{
								Primary: PrimaryReplication{
									PodIndex: 1,
								},
								SyncBinlog: true,
							},
							Replicas: 3,
						},
					},
					wantErr: false,
				},
				{
					by: "invalid HA",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Replication: &Replication{
								Primary: PrimaryReplication{
									PodIndex: 1,
								},
								SyncBinlog: true,
							},
							Galera: &Galera{
								Enabled:        true,
								SST:            "mariabackup",
								ReplicaThreads: 1,
							},
							Replicas: 3,
						},
					},
					wantErr: true,
				},
				{
					by: "fewer replicas required",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Replicas: 4,
						},
					},
					wantErr: true,
				},
				{
					by: "more replicas required",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Galera: &Galera{
								Enabled:        true,
								ReplicaThreads: 4,
							},
							Replicas: 1,
						},
					},
					wantErr: true,
				},
				{
					by: "invalid SST",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Galera: &Galera{
								Enabled:        true,
								SST:            "foo",
								ReplicaThreads: 1,
							},
							Replicas: 3,
						},
					},
					wantErr: true,
				},
				{
					by: "invalid replica threads",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Galera: &Galera{
								Enabled:        true,
								ReplicaThreads: -1,
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
								Primary: PrimaryReplication{
									PodIndex: 4,
								},
								Replica: ReplicaReplication{
									WaitPoint:         func() *WaitPoint { w := WaitPointAfterCommit; return &w }(),
									ConnectionTimeout: &metav1.Duration{Duration: time.Duration(1 * time.Second)},
									ConnectionRetries: 3,
								},
							},
							Replicas: 3,
						},
					},
					wantErr: true,
				},
				{
					by: "invalid replica wait point",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Replication: &Replication{
								Replica: ReplicaReplication{
									WaitPoint: func() *WaitPoint { w := WaitPoint("foo"); return &w }(),
								},
							},
							Replicas: 3,
						},
					},
					wantErr: true,
				},
				{
					by: "invalid GTID",
					mdb: MariaDB{
						ObjectMeta: meta,
						Spec: MariaDBSpec{
							Replication: &Replication{
								Replica: ReplicaReplication{
									Gtid: func() *Gtid { g := Gtid("foo"); return &g }(),
								},
							},
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

		It("Should default Galera", func() {
			storageClassName := "standard"
			mariadb := MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-galera-default-webhook",
					Namespace: testNamespace,
				},
				Spec: MariaDBSpec{
					ContainerTemplate: ContainerTemplate{
						Image: Image{
							Repository: "mariadb",
							Tag:        "10.11.3",
							PullPolicy: corev1.PullIfNotPresent,
						},
					},
					RootPasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secret",
						},
						Key: "root-password",
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
					Replicas: 3,
					Galera: &Galera{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &mariadb)).To(Succeed())
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mariadb), &mariadb)).To(Succeed())

			fiveSeconds := metav1.Duration{Duration: 5 * time.Second}
			oneMinute := metav1.Duration{Duration: 1 * time.Minute}
			fiveMinutes := metav1.Duration{Duration: 5 * time.Minute}
			threeMinutes := metav1.Duration{Duration: 3 * time.Minute}
			defaultStorageClass := "default"
			defaultGalera := &Galera{
				Enabled:        true,
				SST:            SSTMariaBackup,
				ReplicaThreads: 1,
				Agent: &GaleraAgent{
					ContainerTemplate: ContainerTemplate{
						Image: Image{
							Repository: "ghcr.io/mariadb-operator/agent",
							Tag:        "v0.0.2",
							PullPolicy: corev1.PullIfNotPresent,
						},
					},
					Port: 5555,
					KubernetesAuth: &KubernetesAuth{
						Enabled:               true,
						AuthDelegatorRoleName: "mariadb-galera-default-webhook",
					},
					GracefulShutdownTimeout: &fiveSeconds,
				},
				Recovery: &GaleraRecovery{
					Enabled:                 true,
					ClusterHealthyTimeout:   &oneMinute,
					ClusterBootstrapTimeout: &fiveMinutes,
					PodRecoveryTimeout:      &threeMinutes,
					PodSyncTimeout:          &threeMinutes,
				},
				InitContainer: &ContainerTemplate{
					Image: Image{
						Repository: "ghcr.io/mariadb-operator/init",
						Tag:        "v0.0.2",
						PullPolicy: corev1.PullIfNotPresent,
					},
				},
				VolumeClaimTemplate: &corev1.PersistentVolumeClaimSpec{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"storage": resource.MustParse("50Mi"),
						},
					},
					StorageClassName: &defaultStorageClass,
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
				},
			}

			By("Expect MariaDB Galera to be defaulted")
			Expect(mariadb.Spec.Galera).To(Equal(defaultGalera))
		})
	})

	Context("When updating a MariaDB", func() {
		It("Should validate", func() {
			By("Creating MariaDB")
			test := "test"
			storageClassName := "standard"
			mariadb := MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-update-webhook",
					Namespace: testNamespace,
				},
				Spec: MariaDBSpec{
					ContainerTemplate: ContainerTemplate{
						Image: Image{
							Repository: "mariadb",
							Tag:        "10.11.3",
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
							ContainerTemplate: ContainerTemplate{
								Image: Image{
									Repository: "prom/mysqld-exporter",
									Tag:        "v0.14.0",
								},
							},
						},
						ServiceMonitor: ServiceMonitor{
							PrometheusRelease: "prometheus",
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &mariadb)).To(Succeed())

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
						mdb.Spec.BootstrapFrom = nil
					},
					wantErr: false,
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
				Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&mariadb), &mariadb)).To(Succeed())

				patch := client.MergeFrom(mariadb.DeepCopy())
				t.patchFn(&mariadb)

				err := k8sClient.Patch(testCtx, &mariadb, patch)
				if t.wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})

		It("Should validate primary switchover", func() {
			By("Creating MariaDBs")
			noSwitchoverKey := types.NamespacedName{
				Name:      "mariadb-no-switchover-webhook",
				Namespace: testNamespace,
			}
			switchoverKey := types.NamespacedName{
				Name:      "mariadb-switchover-webhook",
				Namespace: testNamespace,
			}
			test := "test"
			storageClassName := "standard"
			mariaDb := MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      noSwitchoverKey.Name,
					Namespace: noSwitchoverKey.Namespace,
				},
				Spec: MariaDBSpec{
					ContainerTemplate: ContainerTemplate{
						Image: Image{
							Repository: "mariadb",
							Tag:        "10.11.3",
						},
					},
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
					Replication: &Replication{
						Primary: PrimaryReplication{
							PodIndex:          0,
							AutomaticFailover: false,
						},
					},
					Replicas: 3,
				},
			}
			mariaDbSwitchover := MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      switchoverKey.Name,
					Namespace: switchoverKey.Namespace,
				},
				Spec: MariaDBSpec{
					ContainerTemplate: ContainerTemplate{
						Image: Image{
							Repository: "mariadb",
							Tag:        "10.11.3",
						},
					},
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
					Replication: &Replication{
						Primary: PrimaryReplication{
							PodIndex:          0,
							AutomaticFailover: false,
						},
					},
					Replicas: 3,
				},
				Status: MariaDBStatus{
					Conditions: []metav1.Condition{
						{
							Type:    ConditionTypePrimarySwitched,
							Status:  metav1.ConditionFalse,
							Reason:  ConditionReasonSwitchPrimary,
							Message: "Switching primary",
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, &mariaDb)).To(Succeed())
			Expect(k8sClient.Create(testCtx, &mariaDbSwitchover)).To(Succeed())

			By("Updating status conditions")
			Expect(k8sClient.Get(testCtx, switchoverKey, &mariaDbSwitchover)).To(Succeed())
			mariaDbSwitchover.Status.Conditions = []metav1.Condition{
				{
					Type:               ConditionTypePrimarySwitched,
					Status:             metav1.ConditionFalse,
					Reason:             ConditionReasonSwitchPrimary,
					Message:            "Switching primary",
					LastTransitionTime: metav1.Now(),
				},
			}
			Expect(k8sClient.Status().Update(testCtx, &mariaDbSwitchover)).To(Succeed())

			// TODO: migrate to Ginkgo v2 and use Ginkgo table tests
			// https://github.com/mariadb-operator/mariadb-operator/issues/3
			tt := []struct {
				by      string
				key     types.NamespacedName
				patchFn func(mdb *MariaDB)
				wantErr bool
			}{
				{
					by:  "Updating primary pod Index",
					key: noSwitchoverKey,
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.Replication.Primary.PodIndex = 1
					},
					wantErr: false,
				},
				{
					by:  "Updating automatic failover",
					key: noSwitchoverKey,
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.Replication.Primary.AutomaticFailover = true
					},
					wantErr: false,
				},
				{
					by:  "Updating primary pod Index when switching",
					key: switchoverKey,
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.Replication.Primary.PodIndex = 1
					},
					wantErr: true,
				},
				{
					by:  "Updating automatic failover when switching",
					key: switchoverKey,
					patchFn: func(mdb *MariaDB) {
						mdb.Spec.Replication.Primary.AutomaticFailover = true
					},
					wantErr: true,
				},
			}

			for _, t := range tt {
				By(t.by)
				var testSwitchover MariaDB
				Expect(k8sClient.Get(testCtx, t.key, &testSwitchover)).To(Succeed())

				patch := client.MergeFrom(testSwitchover.DeepCopy())
				t.patchFn(&testSwitchover)

				err := k8sClient.Patch(testCtx, &testSwitchover, patch)
				if t.wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})
})
