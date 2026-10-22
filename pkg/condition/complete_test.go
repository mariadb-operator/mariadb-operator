package conditions

import (
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testConditioner struct {
	conditions []metav1.Condition
}

func (c *testConditioner) SetCondition(condition metav1.Condition) {
	c.conditions = append(c.conditions, condition)
}

// Condition reasons are asserted as literals on purpose: they are part of the status API that users and
// monitoring match on, so a change to the constant value must show up as a test failure.
func TestSetCompleteWithCronJob(t *testing.T) {
	scheduleTime := metav1.NewTime(time.Now())
	beforeScheduleTime := metav1.NewTime(scheduleTime.Add(-1 * time.Minute))
	afterScheduleTime := metav1.NewTime(scheduleTime.Add(1 * time.Minute))

	tests := []struct {
		name        string
		cronJob     *batchv1.CronJob
		wantStatus  metav1.ConditionStatus
		wantReason  string
		wantMessage string
	}{
		{
			name:        "not scheduled",
			cronJob:     &batchv1.CronJob{},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  "CronJobScheduled",
			wantMessage: "Scheduled",
		},
		{
			name: "scheduled without successful run",
			cronJob: &batchv1.CronJob{
				Status: batchv1.CronJobStatus{
					LastScheduleTime: &scheduleTime,
				},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  "CronJobScheduled",
			wantMessage: "Scheduled",
		},
		{
			name: "running",
			cronJob: &batchv1.CronJob{
				Status: batchv1.CronJobStatus{
					LastScheduleTime:   &scheduleTime,
					LastSuccessfulTime: &beforeScheduleTime,
					Active: []corev1.ObjectReference{
						{
							Name: "job",
						},
					},
				},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  "CronJobRunning",
			wantMessage: "Running",
		},
		{
			name: "failed",
			cronJob: &batchv1.CronJob{
				Status: batchv1.CronJobStatus{
					LastScheduleTime:   &scheduleTime,
					LastSuccessfulTime: &beforeScheduleTime,
				},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  "CronJobFailed",
			wantMessage: "Failed",
		},
		{
			name: "success",
			cronJob: &batchv1.CronJob{
				Status: batchv1.CronJobStatus{
					LastScheduleTime:   &scheduleTime,
					LastSuccessfulTime: &scheduleTime,
				},
			},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  "CronJobSuccess",
			wantMessage: "Success",
		},
		{
			name: "success after being rescheduled",
			cronJob: &batchv1.CronJob{
				Status: batchv1.CronJobStatus{
					LastScheduleTime:   &scheduleTime,
					LastSuccessfulTime: &afterScheduleTime,
				},
			},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  "CronJobSuccess",
			wantMessage: "Success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c testConditioner
			SetCompleteWithCronJob(&c, tt.cronJob)

			assertCondition(t, &c, tt.wantStatus, tt.wantReason, tt.wantMessage)
		})
	}
}

func TestSetCompleteWithJob(t *testing.T) {
	jobWithCondition := func(conditionType batchv1.JobConditionType) *batchv1.Job {
		return &batchv1.Job{
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{
						Type:   conditionType,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}
	}

	tests := []struct {
		name        string
		job         *batchv1.Job
		wantStatus  metav1.ConditionStatus
		wantReason  string
		wantMessage string
	}{
		{
			name:        "running",
			job:         &batchv1.Job{},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  "JobRunning",
			wantMessage: "Running",
		},
		{
			name:        "failed",
			job:         jobWithCondition(batchv1.JobFailed),
			wantStatus:  metav1.ConditionTrue,
			wantReason:  "JobFailed",
			wantMessage: "Failed",
		},
		{
			name:        "suspended",
			job:         jobWithCondition(batchv1.JobSuspended),
			wantStatus:  metav1.ConditionFalse,
			wantReason:  "JobSuspended",
			wantMessage: "Suspended",
		},
		{
			name:        "complete",
			job:         jobWithCondition(batchv1.JobComplete),
			wantStatus:  metav1.ConditionTrue,
			wantReason:  "JobComplete",
			wantMessage: "Success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c testConditioner
			SetCompleteWithJob(&c, tt.job)

			assertCondition(t, &c, tt.wantStatus, tt.wantReason, tt.wantMessage)
		})
	}
}

func assertCondition(t *testing.T, c *testConditioner, status metav1.ConditionStatus, reason, message string) {
	t.Helper()

	if len(c.conditions) != 1 {
		t.Fatalf("expecting exactly one condition to be set, got %d", len(c.conditions))
	}
	condition := c.conditions[0]

	if condition.Type != mariadbv1alpha1.ConditionTypeComplete {
		t.Errorf("expecting condition type to be %q, got %q", mariadbv1alpha1.ConditionTypeComplete, condition.Type)
	}
	if condition.Status != status {
		t.Errorf("expecting condition status to be %q, got %q", status, condition.Status)
	}
	if condition.Reason != reason {
		t.Errorf("expecting condition reason to be %q, got %q", reason, condition.Reason)
	}
	if condition.Message != message {
		t.Errorf("expecting condition message to be %q, got %q", message, condition.Message)
	}
}
