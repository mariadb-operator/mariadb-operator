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

var _ = Describe("SqlJob webhook", func() {
	Context("When updating a SqlJob", func() {
		It("Should validate", func() {
			By("Creating a SqlJob", func() {
				key := types.NamespacedName{
					Name:      "sqljob-webhook",
					Namespace: testNamespace,
				}
				sqlJob := SqlJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key.Name,
						Namespace: key.Namespace,
					},
					Spec: SqlJobSpec{
						DependsOn: []corev1.LocalObjectReference{
							{
								Name: "sqljob-webhook",
							},
						},
						MariaDBRef: MariaDBRef{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-webhook",
							},
							WaitForIt: true,
						},
						Username: "test",
						PasswordSecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test",
							},
							Key: "test",
						},
						Database: func() *string { d := "test"; return &d }(),
						Sql: func() *string {
							sql := `CREATE TABLE IF NOT EXISTS users (
								id bigint PRIMARY KEY AUTO_INCREMENT,
								username varchar(255) NOT NULL,
								email varchar(255) NOT NULL,
								UNIQUE KEY name__unique_idx (username),
								UNIQUE KEY email__unique_idx (email)
							);`
							return &sql
						}(),
					},
				}
				Expect(k8sClient.Create(testCtx, &sqlJob)).To(Succeed())

				tt := []struct {
					by      string
					patchFn func(mdb *SqlJob)
					wantErr bool
				}{
					{
						by: "Updating BackoffLimit",
						patchFn: func(job *SqlJob) {
							job.Spec.BackoffLimit = 20
						},
						wantErr: false,
					},
					{
						by: "Updating RestartPolicy",
						patchFn: func(job *SqlJob) {
							job.Spec.RestartPolicy = corev1.RestartPolicyNever
						},
						wantErr: true,
					},
					{
						by: "Updating Resources",
						patchFn: func(job *SqlJob) {
							job.Spec.Resources = &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("200m"),
								},
							}
						},
						wantErr: true,
					},
					{
						by: "Updating MariaDBRef",
						patchFn: func(job *SqlJob) {
							job.Spec.MariaDBRef.Name = "another-mariadb"
						},
						wantErr: true,
					},
					{
						by: "Updating Username",
						patchFn: func(job *SqlJob) {
							job.Spec.Username = "foo"
						},
						wantErr: true,
					},
					{
						by: "Updating PasswordSecretKeyRef",
						patchFn: func(job *SqlJob) {
							job.Spec.PasswordSecretKeyRef.Name = "foo"
						},
						wantErr: true,
					},
					{
						by: "Updating Database",
						patchFn: func(job *SqlJob) {
							job.Spec.Database = func() *string { d := "foo"; return &d }()
						},
						wantErr: true,
					},
					{
						by: "Updating DependsOn",
						patchFn: func(job *SqlJob) {
							job.Spec.DependsOn = nil
						},
						wantErr: true,
					},
					{
						by: "Updating Sql",
						patchFn: func(job *SqlJob) {
							job.Spec.Sql = func() *string { d := "foo"; return &d }()
						},
						wantErr: true,
					},
					{
						by: "Updating SqlConfigMapKeyRef",
						patchFn: func(job *SqlJob) {
							job.Spec.SqlConfigMapKeyRef = &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "foo",
								},
							}
						},
						wantErr: false,
					},
				}

				for _, t := range tt {
					By(t.by)
					Expect(k8sClient.Get(testCtx, key, &sqlJob)).To(Succeed())

					patch := client.MergeFrom(sqlJob.DeepCopy())
					t.patchFn(&sqlJob)

					err := k8sClient.Patch(testCtx, &sqlJob, patch)
					if t.wantErr {
						Expect(err).To(HaveOccurred())
					} else {
						Expect(err).ToNot(HaveOccurred())
					}
				}
			})
		})
	})
})
