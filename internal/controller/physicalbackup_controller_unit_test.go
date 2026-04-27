package controller

import (
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
