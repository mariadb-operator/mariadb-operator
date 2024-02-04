package controller

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SqlJob controller", func() {
	Context("When creating SqlJobs", func() {
		It("Should reconcile", func() {
			createUsersJob := mariadbv1alpha1.SqlJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sqljob-01-create-table-users",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.SqlJobSpec{
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMdbkey.Name,
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

			insertUsersJob := mariadbv1alpha1.SqlJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sqljob-02-1-insert-users",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.SqlJobSpec{
					DependsOn: []corev1.LocalObjectReference{
						{
							Name: createUsersJob.Name,
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMdbkey.Name,
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

			createReposJob := mariadbv1alpha1.SqlJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sqljob-02-2-create-table-repos",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.SqlJobSpec{
					DependsOn: []corev1.LocalObjectReference{
						{
							Name: createUsersJob.Name,
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMdbkey.Name,
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

			insertReposJob := mariadbv1alpha1.SqlJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sqljob-03-insert-repos",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.SqlJobSpec{
					DependsOn: []corev1.LocalObjectReference{
						{
							Name: createUsersJob.Name,
						},
						{
							Name: createReposJob.Name,
						},
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMdbkey.Name,
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

			By("Expecting SqlJobs to complete eventually")
			for _, j := range sqlJobs {
				Eventually(func() bool {
					var sqlJob mariadbv1alpha1.SqlJob
					if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&j), &sqlJob); err != nil {
						return false
					}
					return sqlJob.IsComplete()
				}, testTimeout, testInterval).Should(BeTrue())
			}

			By("Expecting to create a Job")
			for _, j := range sqlJobs {
				var job batchv1.Job
				Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&j), &job)).To(Succeed())
			}

			By("Deleting SqlJobs")
			for _, sqlJob := range sqlJobs {
				Expect(k8sClient.Delete(testCtx, &sqlJob)).To(Succeed())
			}
		})
	})

	Context("When creating scheduled SqlJob", func() {
		It("Should reconcile", func() {
			scheduledSqlJob := mariadbv1alpha1.SqlJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sqljob-scheduled",
					Namespace: testNamespace,
				},
				Spec: mariadbv1alpha1.SqlJobSpec{
					Schedule: &mariadbv1alpha1.Schedule{
						Cron: "*/1 * * * *",
					},
					MariaDBRef: mariadbv1alpha1.MariaDBRef{
						ObjectReference: corev1.ObjectReference{
							Name: testMdbkey.Name,
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
						sql := `CREATE TABLE IF NOT EXISTS orders (
							id bigint PRIMARY KEY AUTO_INCREMENT,
							email varchar(255) NOT NULL,
							UNIQUE KEY email__unique_idx (email)
						);`
						return &sql
					}(),
				},
			}

			By("Creating SqlJob")
			Expect(k8sClient.Create(testCtx, &scheduledSqlJob)).To(Succeed())

			By("Expecting to create a CronJob eventually")
			Eventually(func() bool {
				var cronJob batchv1.CronJob
				return k8sClient.Get(testCtx, client.ObjectKeyFromObject(&scheduledSqlJob), &cronJob) != nil
			}, testTimeout, testInterval).Should(BeTrue())

			By("Deleting SqlJob")
			Expect(k8sClient.Delete(testCtx, &scheduledSqlJob)).To(Succeed())
		})
	})
})
