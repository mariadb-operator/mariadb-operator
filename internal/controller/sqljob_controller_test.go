package controller

import (
	"reflect"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SqlJob", func() {
	BeforeEach(func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testMdbkey)
	})

	It("should reconcile a Job", func() {
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
				InheritMetadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"k8s.mariadb.com/test": "test",
					},
					Annotations: map[string]string{
						"k8s.mariadb.com/test": "test",
					},
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
				InheritMetadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"k8s.mariadb.com/test": "test",
					},
					Annotations: map[string]string{
						"k8s.mariadb.com/test": "test",
					},
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
				InheritMetadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"k8s.mariadb.com/test": "test",
					},
					Annotations: map[string]string{
						"k8s.mariadb.com/test": "test",
					},
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
		sqlJobs := []mariadbv1alpha1.SqlJob{
			createUsersJob,
			insertUsersJob,
			createReposJob,
		}

		By("Creating SqlJobs")
		for _, sqlJob := range sqlJobs {
			Expect(k8sClient.Create(testCtx, &sqlJob)).To(Succeed())
			sqlJob := sqlJob
			DeferCleanup(func() {
				Expect(k8sClient.Delete(testCtx, &sqlJob)).To(Succeed())
			})
		}

		By("Expecting SqlJobs to complete eventually")
		for _, j := range sqlJobs {
			Eventually(func() bool {
				var sqlJob mariadbv1alpha1.SqlJob
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&j), &sqlJob); err != nil {
					return false
				}
				return sqlJob.IsComplete()
			}, testHighTimeout, testInterval).Should(BeTrue())
		}

		By("Expecting to create a Job")
		for _, sj := range sqlJobs {
			var sqlJob mariadbv1alpha1.SqlJob
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&sj), &sqlJob)).To(Succeed())
			var job batchv1.Job
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(&sqlJob), &job)).To(Succeed())

			By("Expecting Jobs to have metadata")
			Expect(job.ObjectMeta.Labels).NotTo(BeNil())
			Expect(job.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			Expect(job.ObjectMeta.Annotations).NotTo(BeNil())
			Expect(job.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))

			By("Expecting to create a ServiceAccount")
			var svcAcc corev1.ServiceAccount
			key := sqlJob.Spec.JobPodTemplate.ServiceAccountKey(job.ObjectMeta)
			Expect(k8sClient.Get(testCtx, key, &svcAcc)).To(Succeed())

			Expect(svcAcc.ObjectMeta.Labels).NotTo(BeNil())
			Expect(svcAcc.ObjectMeta.Labels).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
			Expect(svcAcc.ObjectMeta.Annotations).NotTo(BeNil())
			Expect(svcAcc.ObjectMeta.Annotations).To(HaveKeyWithValue("k8s.mariadb.com/test", "test"))
		}
	})

	DescribeTable("Creating an SqlJob",
		func(
			resourceName string,
			builderFn func(types.NamespacedName) mariadbv1alpha1.SqlJob,
		) {
			key := types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespace,
			}
			scheduledSqlJob := builderFn(key)
			testScheduledSqlJob(scheduledSqlJob)
		},
		Entry(
			"should reconcile a CronJob",
			"sqljob-scheduled",
			buildScheduledSqlJob,
		),
		Entry(
			"should reconcile a CronJob with history limits",
			"sqljob-scheduled-with-history-limits",
			applyDecoratorChain[mariadbv1alpha1.SqlJob](
				buildScheduledSqlJob,
				decorateSqlJobWithHistoryLimits,
			),
		),
		Entry(
			"should reconcile a CronJob with time zone setting",
			"sqljob-scheduled-with-tz",
			applyDecoratorChain[mariadbv1alpha1.SqlJob](
				buildScheduledSqlJob,
				decorateSqlJobWithTimeZone,
			),
		),
	)
})

func buildScheduledSqlJob(key types.NamespacedName) mariadbv1alpha1.SqlJob {
	return mariadbv1alpha1.SqlJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
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
}

func decorateSqlJobWithHistoryLimits(backup mariadbv1alpha1.SqlJob) mariadbv1alpha1.SqlJob {
	backup.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](5)
	backup.Spec.FailedJobsHistoryLimit = ptr.To[int32](5)
	return backup
}

func decorateSqlJobWithTimeZone(backup mariadbv1alpha1.SqlJob) mariadbv1alpha1.SqlJob {
	backup.Spec.TimeZone = ptr.To[string]("Europe/Sofia")
	return backup
}

func testScheduledSqlJob(scheduledSqlJob mariadbv1alpha1.SqlJob) {
	By("Creating a scheduled SqlJob")
	Expect(k8sClient.Create(testCtx, &scheduledSqlJob)).To(Succeed())
	DeferCleanup(func() {
		Expect(k8sClient.Delete(testCtx, &scheduledSqlJob)).To(Succeed())
	})

	By("Expecting to create a CronJob eventually")
	Eventually(func() bool {
		var cronJob batchv1.CronJob
		err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&scheduledSqlJob), &cronJob)
		if err != nil {
			return false
		}
		isScheduleCorrect := cronJob.Spec.Schedule == scheduledSqlJob.Spec.Schedule.Cron

		if scheduledSqlJob.Spec.SuccessfulJobsHistoryLimit == nil {
			// Kubernetes sets a default of 3 when no limit is specified.
			scheduledSqlJob.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](3)
		}

		if scheduledSqlJob.Spec.FailedJobsHistoryLimit == nil {
			// Kubernetes sets a default of 1 when no limit is specified.
			scheduledSqlJob.Spec.FailedJobsHistoryLimit = ptr.To[int32](1)
		}

		isSuccessfulJobHistoryLimitCorrect :=
			reflect.DeepEqual(cronJob.Spec.SuccessfulJobsHistoryLimit, scheduledSqlJob.Spec.SuccessfulJobsHistoryLimit)
		isFailedJobHistoryLimitCorrect :=
			reflect.DeepEqual(cronJob.Spec.FailedJobsHistoryLimit, scheduledSqlJob.Spec.FailedJobsHistoryLimit)
		isTimeZoneCorrect := reflect.DeepEqual(cronJob.Spec.TimeZone, scheduledSqlJob.Spec.TimeZone)
		return isScheduleCorrect && isSuccessfulJobHistoryLimitCorrect && isFailedJobHistoryLimitCorrect &&
			isTimeZoneCorrect
	}, testHighTimeout, testInterval).Should(BeTrue())

	patch := client.MergeFrom(scheduledSqlJob.DeepCopy())
	scheduledSqlJob.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](7)
	scheduledSqlJob.Spec.FailedJobsHistoryLimit = ptr.To[int32](7)
	By("Updating a scheduled SqlJob's history limits")
	Expect(k8sClient.Patch(testCtx, &scheduledSqlJob, patch)).To(Succeed())

	By("Expecting to update the CronJob history limits eventually")
	Eventually(func() bool {
		var cronJob batchv1.CronJob
		if k8sClient.Get(testCtx, client.ObjectKeyFromObject(&scheduledSqlJob), &cronJob) != nil {
			return false
		}
		isSuccessfulJobHistoryLimitCorrect := *cronJob.Spec.SuccessfulJobsHistoryLimit ==
			*scheduledSqlJob.Spec.SuccessfulJobsHistoryLimit
		isFailedJobHistoryLimitCorrect := *cronJob.Spec.FailedJobsHistoryLimit ==
			*scheduledSqlJob.Spec.FailedJobsHistoryLimit
		return isSuccessfulJobHistoryLimitCorrect && isFailedJobHistoryLimitCorrect
	}, testHighTimeout, testInterval).Should(BeTrue())
}
