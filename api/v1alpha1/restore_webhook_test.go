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

var _ = Describe("Restore webhook", func() {
	Context("When creating a Restore", func() {
		It("Should validate", func() {
			// TODO: migrate to Ginkgo v2 and use Ginkgo table tests
			// https://github.com/mariadb-operator/mariadb-operator/issues/3
			tt := []struct {
				by      string
				restore Restore
				wantErr bool
			}{
				{
					by: "Creating a restore with invalid source 1",
					restore: Restore{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "restore-invalid-source-1",
							Namespace: testNamespace,
						},
						Spec: RestoreSpec{
							RestoreSource: RestoreSource{},
							MariaDBRef: MariaDBRef{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mariadb-webhook",
								},
								WaitForIt: true,
							},
							BackoffLimit: 10,
						},
					},
					wantErr: true,
				},
				{
					by: "Creating a restore with invalid source 2",
					restore: Restore{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "restore-invalid-source-2",
							Namespace: testNamespace,
						},
						Spec: RestoreSpec{
							RestoreSource: RestoreSource{
								FileName: func() *string { s := "foo.sql"; return &s }(),
							},
							MariaDBRef: MariaDBRef{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mariadb-webhook",
								},
								WaitForIt: true,
							},
							BackoffLimit: 10,
						},
					},
					wantErr: true,
				},
				{
					by: "Creating a restore with valid source 1",
					restore: Restore{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "restore-webhook-1",
							Namespace: testNamespace,
						},
						Spec: RestoreSpec{
							RestoreSource: RestoreSource{
								BackupRef: &corev1.LocalObjectReference{
									Name: "backup-webhook",
								},
							},
							MariaDBRef: MariaDBRef{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mariadb-webhook",
								},
								WaitForIt: true,
							},
							BackoffLimit: 10,
						},
					},
					wantErr: false,
				},
				{
					by: "Creating a restore with valid source 2",
					restore: Restore{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "restore-webhook-2",
							Namespace: testNamespace,
						},
						Spec: RestoreSpec{
							RestoreSource: RestoreSource{
								Volume: &corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "pvc-webhook",
									},
								},
							},
							MariaDBRef: MariaDBRef{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mariadb-webhook",
								},
								WaitForIt: true,
							},
							BackoffLimit: 10,
						},
					},
					wantErr: false,
				},
				{
					by: "Creating a restore with valid source 3",
					restore: Restore{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "restore-webhook-3",
							Namespace: testNamespace,
						},
						Spec: RestoreSpec{
							RestoreSource: RestoreSource{
								Volume: &corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "pvc-webhook",
									},
								},
							},
							MariaDBRef: MariaDBRef{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mariadb-webhook",
								},
								WaitForIt: true,
							},
							BackoffLimit: 10,
						},
					},
					wantErr: false,
				},
			}

			for _, t := range tt {
				By(t.by)
				err := k8sClient.Create(testCtx, &t.restore)
				if t.wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})

	Context("When updating a Restore", func() {
		It("Should validate", func() {
			By("Creating a Restore")
			key := types.NamespacedName{
				Name:      "restore-mariadb-webhook",
				Namespace: testNamespace,
			}
			restore := Restore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: RestoreSpec{
					RestoreSource: RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "backup-webhook",
						},
					},
					MariaDBRef: MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					BackoffLimit: 10,
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("100m"),
						},
					},
					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			}
			Expect(k8sClient.Create(testCtx, &restore)).To(Succeed())

			// TODO: migrate to Ginkgo v2 and use Ginkgo table tests
			// https://github.com/mariadb-operator/mariadb-operator/issues/3
			tt := []struct {
				by      string
				patchFn func(mdb *Restore)
				wantErr bool
			}{
				{
					by: "Updating BackoffLimit",
					patchFn: func(rmdb *Restore) {
						rmdb.Spec.BackoffLimit = 20
					},
					wantErr: false,
				},
				{
					by: "Updating RestartPolicy",
					patchFn: func(rmdb *Restore) {
						rmdb.Spec.RestartPolicy = corev1.RestartPolicyNever
					},
					wantErr: true,
				},
				{
					by: "Updating Resources",
					patchFn: func(rmdb *Restore) {
						rmdb.Spec.Resources = &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("200m"),
							},
						}
					},
					wantErr: true,
				},
				{
					by: "Updating MariaDBRef",
					patchFn: func(rmdb *Restore) {
						rmdb.Spec.MariaDBRef.Name = "another-mariadb"
					},
					wantErr: true,
				},
				{
					by: "Updating BackupRef source",
					patchFn: func(rmdb *Restore) {
						rmdb.Spec.RestoreSource.BackupRef.Name = "another-backup"
					},
					wantErr: true,
				},
				{
					by: "Init Volume source",
					patchFn: func(rmdb *Restore) {
						rmdb.Spec.RestoreSource.Volume = &corev1.VolumeSource{
							NFS: &corev1.NFSVolumeSource{
								Server: "nas.local",
								Path:   "/volume/foo",
							},
						}
					},
					wantErr: false,
				},
				{
					by: "Init FileName source",
					patchFn: func(rmdb *Restore) {
						rmdb.Spec.RestoreSource.FileName = func() *string {
							f := "backup.sql"
							return &f
						}()
					},
					wantErr: false,
				},
			}

			for _, t := range tt {
				By(t.by)
				Expect(k8sClient.Get(testCtx, key, &restore)).To(Succeed())

				patch := client.MergeFrom(restore.DeepCopy())
				t.patchFn(&restore)

				err := k8sClient.Patch(testCtx, &restore, patch)
				if t.wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})
})
