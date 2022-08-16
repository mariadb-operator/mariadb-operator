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

var _ = Describe("BackupMariaDB webhook", func() {
	Context("When updating a BackupMariaDB", func() {
		It("Should validate", func() {
			By("Creating BackupMariaDB")
			key := types.NamespacedName{
				Name:      "backup-mariadb-webhook",
				Namespace: testNamespace,
			}
			backup := BackupMariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: BackupMariaDBSpec{
					Storage: Storage{
						ClassName: "standard",
						Size:      resource.MustParse("100Mi"),
					},
					MariaDBRef: corev1.LocalObjectReference{
						Name: "mariadb-webhook",
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
			Expect(k8sClient.Create(testCtx, &backup)).To(Succeed())

			// TODO: migrate to Ginkgo v2 and use Ginkgo table tests
			// https://github.com/mmontes11/mariadb-operator/issues/3
			tt := []struct {
				by      string
				patchFn func(mdb *BackupMariaDB)
				wantErr bool
			}{
				{
					by: "Updating BackoffLimit",
					patchFn: func(bmdb *BackupMariaDB) {
						bmdb.Spec.BackoffLimit = 20
					},
					wantErr: false,
				},
				{
					by: "Updating Storage",
					patchFn: func(bmdb *BackupMariaDB) {
						bmdb.Spec.Storage.ClassName = "fast-storage"
					},
					wantErr: true,
				},
				{
					by: "Updating MariaDBRef",
					patchFn: func(bmdb *BackupMariaDB) {
						bmdb.Spec.MariaDBRef.Name = "another-mariadb"
					},
					wantErr: true,
				},
				{
					by: "Updating RestartPolicy",
					patchFn: func(bmdb *BackupMariaDB) {
						bmdb.Spec.RestartPolicy = corev1.RestartPolicyNever
					},
					wantErr: true,
				},
				{
					by: "Updating Resources",
					patchFn: func(bmdb *BackupMariaDB) {
						bmdb.Spec.Resources = &corev1.ResourceRequirements{
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
				Expect(k8sClient.Get(testCtx, key, &backup)).To(Succeed())

				patch := client.MergeFrom(backup.DeepCopy())
				t.patchFn(&backup)

				err := k8sClient.Patch(testCtx, &backup, patch)
				if t.wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})
})
