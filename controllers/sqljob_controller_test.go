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
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("SqlJob controller", func() {
	Context("When creating SqlJobs", func() {
		It("Should reconcile", func() {
			createUsersJobKey := types.NamespacedName{
				Name:      "sqljob-01-create-table-users",
				Namespace: testNamespace,
			}
			createUsersJob := mariadbv1alpha1.SqlJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      createUsersJobKey.Name,
					Namespace: createUsersJobKey.Namespace,
				},
				Spec: mariadbv1alpha1.SqlJobSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					Username: testUser,
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Database: &testDatabase,
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

			insertUsersKey := types.NamespacedName{
				Name:      "sqljob-02-1-insert-users",
				Namespace: testNamespace,
			}
			insertUsersJob := mariadbv1alpha1.SqlJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      insertUsersKey.Name,
					Namespace: insertUsersKey.Namespace,
				},
				Spec: mariadbv1alpha1.SqlJobSpec{
					DependsOn: []corev1.LocalObjectReference{
						{
							Name: createUsersJob.Name,
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					Username: testUser,
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Database: &testDatabase,
					Sql: func() *string {
						sql := `INSERT INTO users(username, email) VALUES('mmontes11','mariadb-operator@proton.me') 
						ON DUPLICATE KEY UPDATE username='mmontes11';`
						return &sql
					}(),
				},
			}

			createReposKey := types.NamespacedName{
				Name:      "sqljob-02-2-create-table-repos",
				Namespace: testNamespace,
			}
			createReposJob := mariadbv1alpha1.SqlJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      createReposKey.Name,
					Namespace: createReposKey.Namespace,
				},
				Spec: mariadbv1alpha1.SqlJobSpec{
					DependsOn: []corev1.LocalObjectReference{
						{
							Name: createUsersJob.Name,
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					Username: testUser,
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Database: &testDatabase,
					Sql: func() *string {
						sql := `CREATE TABLE IF NOT EXISTS repos (
							id bigint PRIMARY KEY AUTO_INCREMENT,
							name varchar(255) NOT NULL,
							owner_id bigint NOT NULL,
							UNIQUE KEY name__unique_idx (name),
							FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
						);`
						return &sql
					}(),
				},
			}

			insertReposKey := types.NamespacedName{
				Name:      "sqljob-03-insert-repos",
				Namespace: testNamespace,
			}
			insertReposJob := mariadbv1alpha1.SqlJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      insertReposKey.Name,
					Namespace: insertReposKey.Namespace,
				},
				Spec: mariadbv1alpha1.SqlJobSpec{
					DependsOn: []corev1.LocalObjectReference{
						{
							Name: createUsersJob.Name,
						},
						{
							Name: createReposKey.Name,
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testMariaDbName,
						},
						WaitForIt: true,
					},
					Username: testUser,
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: testPwdKey.Name,
						},
						Key: testPwdSecretKey,
					},
					Database: &testDatabase,
					Sql: func() *string {
						sql := `    INSERT INTO repos(name, owner_id) VALUES('mariadb-operator', (SELECT id FROM users WHERE username = 'mmontes11'))
						ON DUPLICATE KEY UPDATE name='mariadb-operator';`
						return &sql
					}(),
				},
			}

			sqlJobKeys := []types.NamespacedName{
				createUsersJobKey,
				insertUsersKey,
				createReposKey,
				insertReposKey,
			}
			sqlJobs := []mariadbv1alpha1.SqlJob{
				createUsersJob,
				insertUsersJob,
				createReposJob,
				insertReposJob,
			}

			By("Creating SqlJobs")
			for _, sqlJob := range sqlJobs {
				Expect(k8sClient.Create(testCtx, &sqlJob)).To(Succeed())
			}

			By("Expecting SqlJobs to be complete eventually")
			for _, key := range sqlJobKeys {
				Eventually(func() bool {
					var sqlJob mariadbv1alpha1.SqlJob
					if err := k8sClient.Get(testCtx, key, &sqlJob); err != nil {
						return false
					}
					return sqlJob.IsComplete()
				}, testTimeout, testInterval).Should(BeTrue())
			}

			By("Expecting to create a Job eventually")
			for _, key := range sqlJobKeys {
				var job batchv1.Job
				Expect(k8sClient.Get(testCtx, key, &job)).To(Succeed())
			}

			By("Deleting SqlJobs")
			for _, sqlJob := range sqlJobs {
				Expect(k8sClient.Delete(testCtx, &sqlJob)).To(Succeed())
			}
		})
	})
})
