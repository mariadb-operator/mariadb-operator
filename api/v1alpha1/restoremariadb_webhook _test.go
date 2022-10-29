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

var _ = Describe("RestoreMariaDB webhook", func() {
	Context("When updating a RestoreMariaDB", func() {
		It("Should validate", func() {
			By("Creating RestoreMariaDB")
			key := types.NamespacedName{
				Name:      "restore-mariadb-webhook",
				Namespace: testNamespace,
			}
			restore := RestoreMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: RestoreMariaDBSpec{
					MariaDBRef: MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					BackupRef: BackupMariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "backup-webhook",
						},
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
			// https://github.com/mmontes11/mariadb-operator/issues/3
			tt := []struct {
				by      string
				patchFn func(mdb *RestoreMariaDB)
				wantErr bool
			}{
				{
					by: "Updating BackoffLimit",
					patchFn: func(rmdb *RestoreMariaDB) {
						rmdb.Spec.BackoffLimit = 20
					},
					wantErr: false,
				},
				{
					by: "Updating MariaDBRef",
					patchFn: func(rmdb *RestoreMariaDB) {
						rmdb.Spec.MariaDBRef.Name = "another-mariadb"
					},
					wantErr: true,
				},
				{
					by: "Updating BackupRef",
					patchFn: func(rmdb *RestoreMariaDB) {
						rmdb.Spec.BackupRef.Name = "another-backup"
					},
					wantErr: true,
				},
				{
					by: "Updating RestartPolicy",
					patchFn: func(rmdb *RestoreMariaDB) {
						rmdb.Spec.RestartPolicy = corev1.RestartPolicyNever
					},
					wantErr: true,
				},
				{
					by: "Updating Resources",
					patchFn: func(rmdb *RestoreMariaDB) {
						rmdb.Spec.Resources = &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("200m"),
							},
						}
					},
					wantErr: true,
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
