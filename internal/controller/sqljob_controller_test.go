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

var _ = Describe("SqlJob", Label("basic"), func() {
	BeforeEach(func() {
		By("Waiting for MariaDB to be ready")
		expectMariadbReady(testCtx, k8sClient, testMdbkey)
	})

	It("should reconcile a Job", func() {
		createUsersJob := mariadbv1alpha1.SQLJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sqljob-01-create-table-users",
				Namespace: testNamespace,
			},
			Spec: mariadbv1alpha1.SQLJobSpec{
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
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
				PasswordSecretKeyRef: mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: testPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
				Database:               &testDatabase,
				TLSCACertSecretRef:     testTLSClientCARef,
				TLSClientCertSecretRef: testTLSClientCertRef,
				SQL: func() *string {
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

		insertUsersJob := mariadbv1alpha1.SQLJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sqljob-02-1-insert-users",
				Namespace: testNamespace,
			},
			Spec: mariadbv1alpha1.SQLJobSpec{
				DependsOn: []mariadbv1alpha1.LocalObjectReference{
					{
						Name: createUsersJob.Name,
					},
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
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
				PasswordSecretKeyRef: mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: testPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
				Database: &testDatabase,
				SQL: func() *string {
					sql := `INSERT INTO users(username, email) VALUES('mmontes11','mariadb-operator@proton.me') 
						ON DUPLICATE KEY UPDATE username='mmontes11';`
					return &sql
				}(),
			},
		}

		createReposJob := mariadbv1alpha1.SQLJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sqljob-02-2-create-table-repos",
				Namespace: testNamespace,
			},
			Spec: mariadbv1alpha1.SQLJobSpec{
				DependsOn: []mariadbv1alpha1.LocalObjectReference{
					{
						Name: createUsersJob.Name,
					},
				},
				MariaDBRef: mariadbv1alpha1.MariaDBRef{
					ObjectReference: mariadbv1alpha1.ObjectReference{
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
				PasswordSecretKeyRef: mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: testPwdKey.Name,
					},
					Key: testPwdSecretKey,
				},
				Database: &testDatabase,
				SQL: func() *string {
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
		sqlJobs := []mariadbv1alpha1.SQLJob{
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
				var sqlJob mariadbv1alpha1.SQLJob
				if err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&j), &sqlJob); err != nil {
					return false
				}
				return sqlJob.IsComplete()
			}, testHighTimeout, testInterval).Should(BeTrue())
		}

		By("Expecting to create a Job")
		for _, sj := range sqlJobs {
			var sqlJob mariadbv1alpha1.SQLJob
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
			key := sqlJob.Spec.ServiceAccountKey(job.ObjectMeta)
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
			builderFn func(types.NamespacedName) mariadbv1alpha1.SQLJob,
		) {
			key := types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespace,
			}
			scheduledSQLJob := builderFn(key)
			testScheduledSQLJob(scheduledSQLJob)
		},
		Entry(
			"should reconcile a CronJob",
			"sqljob-scheduled",
			buildScheduledSQLJob,
		),
		Entry(
			"should reconcile a CronJob with history limits",
			"sqljob-scheduled-with-history-limits",
			applyDecoratorChain(
				buildScheduledSQLJob,
				decorateSQLJobWithHistoryLimits,
			),
		),
		Entry(
			"should reconcile a CronJob with time zone setting",
			"sqljob-scheduled-with-tz",
			applyDecoratorChain(
				buildScheduledSQLJob,
				decorateSQLJobWithTimeZone,
			),
		),
	)
})

func buildScheduledSQLJob(key types.NamespacedName) mariadbv1alpha1.SQLJob {
	return mariadbv1alpha1.SQLJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: mariadbv1alpha1.SQLJobSpec{
			Schedule: &mariadbv1alpha1.Schedule{
				Cron: "*/1 * * * *",
			},
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: mariadbv1alpha1.ObjectReference{
					Name: testMdbkey.Name,
				},
				WaitForIt: true,
			},
			Username: testUser,
			PasswordSecretKeyRef: mariadbv1alpha1.SecretKeySelector{
				LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
					Name: testPwdKey.Name,
				},
				Key: testPwdSecretKey,
			},
			Database: &testDatabase,
			SQL: func() *string {
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

func decorateSQLJobWithHistoryLimits(backup mariadbv1alpha1.SQLJob) mariadbv1alpha1.SQLJob {
	backup.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](5)
	backup.Spec.FailedJobsHistoryLimit = ptr.To[int32](5)
	return backup
}

func decorateSQLJobWithTimeZone(backup mariadbv1alpha1.SQLJob) mariadbv1alpha1.SQLJob {
	backup.Spec.TimeZone = ptr.To[string]("Europe/Sofia")
	return backup
}

func testScheduledSQLJob(scheduledSQLJob mariadbv1alpha1.SQLJob) {
	By("Creating a scheduled SqlJob")
	Expect(k8sClient.Create(testCtx, &scheduledSQLJob)).To(Succeed())
	DeferCleanup(func() {
		Expect(k8sClient.Delete(testCtx, &scheduledSQLJob)).To(Succeed())
	})

	By("Expecting to create a CronJob eventually")
	Eventually(func() bool {
		var cronJob batchv1.CronJob
		err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(&scheduledSQLJob), &cronJob)
		if err != nil {
			return false
		}
		isScheduleCorrect := cronJob.Spec.Schedule == scheduledSQLJob.Spec.Schedule.Cron

		if scheduledSQLJob.Spec.SuccessfulJobsHistoryLimit == nil {
			// Kubernetes sets a default of 3 when no limit is specified.
			scheduledSQLJob.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](3)
		}

		if scheduledSQLJob.Spec.FailedJobsHistoryLimit == nil {
			// Kubernetes sets a default of 1 when no limit is specified.
			scheduledSQLJob.Spec.FailedJobsHistoryLimit = ptr.To[int32](1)
		}

		return isScheduleCorrect && assertSQLJobCronJobTemplateSpecsEqual(cronJob, scheduledSQLJob)
	}, testHighTimeout, testInterval).Should(BeTrue())

	patch := client.MergeFrom(scheduledSQLJob.DeepCopy())
	scheduledSQLJob.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](7)
	scheduledSQLJob.Spec.FailedJobsHistoryLimit = ptr.To[int32](7)
	scheduledSQLJob.Spec.TimeZone = ptr.To[string]("Europe/Madrid")
	By("Updating a scheduled SqlJob's history limits and time zone")
	Expect(k8sClient.Patch(testCtx, &scheduledSQLJob, patch)).To(Succeed())

	By("Expecting to update the CronJob history limits and time zone eventually")
	Eventually(func() bool {
		var cronJob batchv1.CronJob
		if k8sClient.Get(testCtx, client.ObjectKeyFromObject(&scheduledSQLJob), &cronJob) != nil {
			return false
		}
		return assertSQLJobCronJobTemplateSpecsEqual(cronJob, scheduledSQLJob)
	}, testHighTimeout, testInterval).Should(BeTrue())
}

func assertSQLJobCronJobTemplateSpecsEqual(cronJob batchv1.CronJob, sqlJob mariadbv1alpha1.SQLJob) bool {
	isSuccessfulJobHistoryLimitCorrect :=
		reflect.DeepEqual(cronJob.Spec.SuccessfulJobsHistoryLimit, sqlJob.Spec.SuccessfulJobsHistoryLimit)
	isFailedJobHistoryLimitCorrect :=
		reflect.DeepEqual(cronJob.Spec.FailedJobsHistoryLimit, sqlJob.Spec.FailedJobsHistoryLimit)
	isTimeZoneCorrect := reflect.DeepEqual(cronJob.Spec.TimeZone, sqlJob.Spec.TimeZone)
	return isSuccessfulJobHistoryLimitCorrect && isFailedJobHistoryLimitCorrect && isTimeZoneCorrect
}
