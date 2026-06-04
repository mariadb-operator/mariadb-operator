package controller

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestShouldReconcilePhysicalBackupAllowsReplicaRecoveryDuringPendingUpdate(t *testing.T) {
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mdb",
			Namespace: "default",
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			Conditions: []metav1.Condition{
				{
					Type:   mariadbv1alpha1.ConditionTypeUpdated,
					Status: metav1.ConditionFalse,
					Reason: mariadbv1alpha1.ConditionReasonPendingUpdate,
				},
				{
					Type:   mariadbv1alpha1.ConditionTypeReplicaRecovered,
					Status: metav1.ConditionFalse,
					Reason: mariadbv1alpha1.ConditionReasonReplicaRecovering,
				},
			},
		},
	}
	recoveryKey := mariadb.PhysicalBackupReplicaRecoveryKey()
	backup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      recoveryKey.Name,
			Namespace: recoveryKey.Namespace,
		},
	}

	if !shouldReconcilePhysicalBackup(backup, mariadb, logr.Discard()) {
		t.Fatal("expected replica recovery PhysicalBackup to reconcile during pending update")
	}
}

func TestShouldReconcilePhysicalBackupSkipsRegularBackupDuringPendingUpdate(t *testing.T) {
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mdb",
			Namespace: "default",
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			Conditions: []metav1.Condition{
				{
					Type:   mariadbv1alpha1.ConditionTypeUpdated,
					Status: metav1.ConditionFalse,
					Reason: mariadbv1alpha1.ConditionReasonPendingUpdate,
				},
				{
					Type:   mariadbv1alpha1.ConditionTypeReplicaRecovered,
					Status: metav1.ConditionFalse,
					Reason: mariadbv1alpha1.ConditionReasonReplicaRecovering,
				},
			},
		},
	}
	backup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "physicalbackup-tpl",
			Namespace: "default",
		},
	}

	if shouldReconcilePhysicalBackup(backup, mariadb, logr.Discard()) {
		t.Fatal("expected regular PhysicalBackup to stay blocked during pending update")
	}
}

func TestShouldReconcilePhysicalBackupSkipsRecoveryNameWhenNotRecoveringReplicas(t *testing.T) {
	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mdb",
			Namespace: "default",
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			Conditions: []metav1.Condition{
				{
					Type:   mariadbv1alpha1.ConditionTypeUpdated,
					Status: metav1.ConditionFalse,
					Reason: mariadbv1alpha1.ConditionReasonPendingUpdate,
				},
			},
		},
	}
	recoveryKey := mariadb.PhysicalBackupReplicaRecoveryKey()
	backup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      recoveryKey.Name,
			Namespace: recoveryKey.Namespace,
		},
	}

	if shouldReconcilePhysicalBackup(backup, mariadb, logr.Discard()) {
		t.Fatal("expected recovery-named PhysicalBackup to stay blocked when replica recovery is not active")
	}
}

func TestReconcileTemplateScheduledRetriesMissingImmediateBackupArtifact(t *testing.T) {
	now := time.Now().Add(-missingPhysicalBackupArtifactRetryDelay - time.Second)
	backup := &mariadbv1alpha1.PhysicalBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "physicalbackup",
			Namespace: "default",
		},
		Spec: mariadbv1alpha1.PhysicalBackupSpec{
			Schedule: &mariadbv1alpha1.PhysicalBackupSchedule{
				Cron:      "",
				Immediate: ptr.To(true),
			},
		},
		Status: mariadbv1alpha1.PhysicalBackupStatus{
			LastScheduleCheckTime: &metav1.Time{Time: now},
			LastScheduleTime:      &metav1.Time{Time: now},
		},
	}

	called := false
	reconciler := &PhysicalBackupReconciler{}
	result, err := reconciler.reconcileTemplateScheduled(context.Background(), backup, 0,
		func(_ time.Time, _ cron.Schedule) (ctrl.Result, error) {
			called = true
			return ctrl.Result{}, nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("expected zero result, got %+v", result)
	}
	if !called {
		t.Fatal("expected missing immediate backup artifact to be retried")
	}
}

func TestMissingPhysicalBackupArtifactRetryAfterWaitsForRecentSchedule(t *testing.T) {
	now := time.Now()
	backup := &mariadbv1alpha1.PhysicalBackup{
		Spec: mariadbv1alpha1.PhysicalBackupSpec{
			Schedule: &mariadbv1alpha1.PhysicalBackupSchedule{
				Immediate: ptr.To(true),
			},
		},
		Status: mariadbv1alpha1.PhysicalBackupStatus{
			LastScheduleTime: &metav1.Time{Time: now.Add(-10 * time.Second)},
		},
	}

	retryAfter := missingPhysicalBackupArtifactRetryAfter(backup, 0, now)
	if retryAfter == nil {
		t.Fatal("expected retry delay")
	}
	if *retryAfter <= 0 {
		t.Fatalf("expected positive retry delay, got %v", *retryAfter)
	}
}

func TestMissingPhysicalBackupArtifactRetryAfterIgnoresExistingArtifacts(t *testing.T) {
	now := time.Now().Add(-missingPhysicalBackupArtifactRetryDelay - time.Second)
	backup := &mariadbv1alpha1.PhysicalBackup{
		Spec: mariadbv1alpha1.PhysicalBackupSpec{
			Schedule: &mariadbv1alpha1.PhysicalBackupSchedule{
				Immediate: ptr.To(true),
			},
		},
		Status: mariadbv1alpha1.PhysicalBackupStatus{
			LastScheduleTime: &metav1.Time{Time: now},
		},
	}

	if retryAfter := missingPhysicalBackupArtifactRetryAfter(backup, 1, time.Now()); retryAfter != nil {
		t.Fatalf("expected no retry with existing artifacts, got %v", *retryAfter)
	}
}
