package controller

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileAndWaitForInitJobSetsScaleOutErrorForUnschedulableJob(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding batch scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			Conditions: []metav1.Condition{
				{
					Type:    mariadbv1alpha1.ConditionTypeScaledOut,
					Status:  metav1.ConditionFalse,
					Reason:  mariadbv1alpha1.ConditionReasonScalingOut,
					Message: "Scaling out",
				},
			},
		},
	}
	jobKey := mariadb.PhysicalBackupInitJobKey(1)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobKey.Name,
			Namespace: jobKey.Namespace,
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-1-pb-init-abcde",
			Namespace: jobKey.Namespace,
			Labels: map[string]string{
				"batch.kubernetes.io/job-name": jobKey.Name,
			},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{
					Type:    corev1.PodScheduled,
					Status:  corev1.ConditionFalse,
					Reason:  corev1.PodReasonUnschedulable,
					Message: "0/5 nodes are available: 1 didn't match pod anti-affinity rules.",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		WithObjects(mariadb, job, pod).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	result, err := reconciler.reconcileAndWaitForInitJob(context.Background(), mariadb, jobKey, 1, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("expected requeue after 30s, got %v", result.RequeueAfter)
	}

	var updated mariadbv1alpha1.MariaDB
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &updated); err != nil {
		t.Fatalf("error getting MariaDB: %v", err)
	}
	if updated.ScalingOutError() == nil {
		t.Fatalf("expected scale out error to be set")
	}
}

func TestReconcilePhysicalBackupInitErrorPreservesCustomError(t *testing.T) {
	reconciler := &MariaDBReconciler{}
	mariadb := &mariadbv1alpha1.MariaDB{
		Status: mariadbv1alpha1.MariaDBStatus{
			Conditions: []metav1.Condition{
				{
					Type:    mariadbv1alpha1.ConditionTypeInitialized,
					Status:  metav1.ConditionFalse,
					Reason:  mariadbv1alpha1.ConditionReasonInitError,
					Message: "Init error: PhysicalBackup init Job is unschedulable",
				},
			},
		},
	}

	result, err := reconciler.reconcilePhysicalBackupInitError(context.Background(), mariadb, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("expected requeue after 30s, got %v", result.RequeueAfter)
	}
}

func TestReconcileScaleOutErrorPreservesCustomError(t *testing.T) {
	reconciler := &MariaDBReconciler{}
	mariadb := &mariadbv1alpha1.MariaDB{
		Status: mariadbv1alpha1.MariaDBStatus{
			Conditions: []metav1.Condition{
				{
					Type:    mariadbv1alpha1.ConditionTypeScaledOut,
					Status:  metav1.ConditionFalse,
					Reason:  mariadbv1alpha1.ConditionReasonScaleOutError,
					Message: "Scale out error: non-retriable custom error",
				},
			},
		},
	}

	result, err := reconciler.reconcileScaleOutError(context.Background(), mariadb, 1, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("expected requeue after 30s, got %v", result.RequeueAfter)
	}
}

func TestReconcileScaleOutErrorRetriesUnschedulableInitJobError(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
				ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
					Replica: mariadbv1alpha1.ReplicaReplication{
						ReplicaBootstrapFrom: &mariadbv1alpha1.ReplicaBootstrapFrom{
							PhysicalBackupTemplateRef: mariadbv1alpha1.LocalObjectReference{
								Name: "physicalbackup-tpl",
							},
						},
					},
				},
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			Conditions: []metav1.Condition{
				{
					Type:   mariadbv1alpha1.ConditionTypeScaledOut,
					Status: metav1.ConditionFalse,
					Reason: mariadbv1alpha1.ConditionReasonScaleOutError,
					Message: "Scale out error: PhysicalBackup init Job 'mariadb-1-pb-init' is unschedulable: " +
						"Job Pod 'mariadb-1-pb-init-abcde' is unschedulable: 0/1 nodes are available: " +
						"pod has unbound immediate PersistentVolumeClaims.",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		WithObjects(mariadb).
		Build()

	reconciler := &MariaDBReconciler{Client: fakeClient}

	result, err := reconciler.reconcileScaleOutError(context.Background(), mariadb, 1, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("expected no requeue for retriable init-job unschedulable error, got %v", result)
	}
}

func TestWaitForInitJobCompleteSetsReplicaRecoveryErrorForUnschedulableJob(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding batch scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}

	mariadb := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb",
			Namespace: "test",
		},
	}
	jobKey := mariadb.PhysicalBackupInitJobKey(1)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobKey.Name,
			Namespace: jobKey.Namespace,
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-1-pb-init-abcde",
			Namespace: jobKey.Namespace,
			Labels: map[string]string{
				"batch.kubernetes.io/job-name": jobKey.Name,
			},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{
					Type:    corev1.PodScheduled,
					Status:  corev1.ConditionFalse,
					Reason:  corev1.PodReasonUnschedulable,
					Message: "0/5 nodes are available: 1 didn't match pod anti-affinity rules.",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
		WithObjects(mariadb, job, pod).
		Build()

	reconciler := &MariaDBReconciler{
		Client: fakeClient,
	}

	result, err := reconciler.waitForInitJobComplete(context.Background(), mariadb, jobKey, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 30*time.Second {
		t.Fatalf("expected requeue after 30s, got %v", result.RequeueAfter)
	}

	var updated mariadbv1alpha1.MariaDB
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(mariadb), &updated); err != nil {
		t.Fatalf("error getting MariaDB: %v", err)
	}
	if updated.ReplicaRecoveryError() == nil {
		t.Fatalf("expected replica recovery error to be set")
	}
}
