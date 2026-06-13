package controller

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIsReplicaBootstrapScaleOutRecovery(t *testing.T) {
	testCases := map[string]struct {
		annotations map[string]string
		fromIndex   int
		pvcUIDs     map[int]string
		want        bool
	}{
		"lost tail replica pvc recreated": {
			annotations: map[string]string{
				storagePVCUIDAnnotationKey(2): "old-tail-uid",
			},
			fromIndex: 2,
			pvcUIDs: map[int]string{
				2: "new-tail-uid",
			},
			want: true,
		},
		"new replica from user scale out": {
			annotations: map[string]string{
				storagePVCUIDAnnotationKey(0): "primary-uid",
				storagePVCUIDAnnotationKey(1): "replica-uid",
			},
			fromIndex: 2,
			pvcUIDs: map[int]string{
				2: "new-tail-uid",
			},
			want: false,
		},
		"tracked tail pvc unchanged": {
			annotations: map[string]string{
				storagePVCUIDAnnotationKey(2): "tail-uid",
			},
			fromIndex: 2,
			pvcUIDs: map[int]string{
				2: "tail-uid",
			},
			want: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			mariadb := &mariadbv1alpha1.MariaDB{}
			mariadb.Annotations = tc.annotations

			got := isReplicaBootstrapScaleOutRecovery(mariadb, tc.fromIndex, tc.pvcUIDs)
			if got != tc.want {
				t.Fatalf("unexpected recovery scale-out detection: got %t, want %t", got, tc.want)
			}
		})
	}
}

func TestIsScalingOutContinuesDuringReplicaRecoveryScaleOut(t *testing.T) {
	reconciler := &MariaDBReconciler{}
	mariadb := &mariadbv1alpha1.MariaDB{
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 3,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
			},
		},
		Status: mariadbv1alpha1.MariaDBStatus{
			Conditions: []metav1.Condition{
				{
					Type:   mariadbv1alpha1.ConditionTypeScaledOut,
					Status: metav1.ConditionFalse,
				},
				{
					Type:   mariadbv1alpha1.ConditionTypeReplicaRecovered,
					Status: metav1.ConditionFalse,
				},
			},
		},
	}
	sts := &appsv1.StatefulSet{
		Status: appsv1.StatefulSetStatus{
			Replicas:      2,
			ReadyReplicas: 2,
		},
	}

	isScalingOut, err := reconciler.isScalingOut(mariadb, sts, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isScalingOut {
		t.Fatalf("expected ongoing scale out to continue while replica recovery condition is false")
	}
}

func TestReplicaBootstrapScaleOutPendingAllowsSwitchoverBlockedScaleOut(t *testing.T) {
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
			Annotations: map[string]string{
				storagePVCUIDAnnotationKey(0): "primary-uid",
				storagePVCUIDAnnotationKey(1): "old-tail-uid",
			},
		},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Replicas: 2,
			Replication: &mariadbv1alpha1.Replication{
				Enabled: true,
				ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
					Primary: mariadbv1alpha1.PrimaryReplication{
						PodIndex: ptr.To(1),
					},
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
			CurrentPrimaryPodIndex: ptr.To(0),
		},
	}
	sts := &appsv1.StatefulSet{
		Status: appsv1.StatefulSetStatus{
			Replicas:      1,
			ReadyReplicas: 1,
		},
	}
	reconciler := &MariaDBReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(
				mariadb,
				scaleOutPVC("mariadb", "test", 0, "primary-uid"),
				scaleOutPVC("mariadb", "test", 1, "new-tail-uid"),
			).
			Build(),
	}

	pending, err := reconciler.isReplicaBootstrapScaleOutPending(context.Background(), mariadb, sts, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pending {
		t.Fatalf("expected replica bootstrap scale out to be pending")
	}

	isScalingOut, err := reconciler.isScalingOut(mariadb, sts, pending)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isScalingOut {
		t.Fatalf("expected scale out to proceed before switchover")
	}
}

func TestHasCompletedScaleOutInitJobPVCs(t *testing.T) {
	testCases := map[string]struct {
		replicas int32
		pvcs     []*corev1.PersistentVolumeClaim
		jobs     []*batchv1.Job
		want     bool
	}{
		"completed matching init job allows restored pvc": {
			replicas: 2,
			pvcs: []*corev1.PersistentVolumeClaim{
				scaleOutPVC("mariadb", "test", 1, "restored-uid"),
			},
			jobs: []*batchv1.Job{
				completedScaleOutInitJob("mariadb", "test", 1, "restored-uid"),
			},
			want: true,
		},
		"missing pvc does not count as restored": {
			replicas: 2,
			want:     false,
		},
		"running init job does not allow existing pvc": {
			replicas: 2,
			pvcs: []*corev1.PersistentVolumeClaim{
				scaleOutPVC("mariadb", "test", 1, "restored-uid"),
			},
			jobs: []*batchv1.Job{
				scaleOutInitJob("mariadb", "test", 1, "restored-uid", false),
			},
			want: false,
		},
		"mismatched init job pvc uid does not allow existing pvc": {
			replicas: 2,
			pvcs: []*corev1.PersistentVolumeClaim{
				scaleOutPVC("mariadb", "test", 1, "restored-uid"),
			},
			jobs: []*batchv1.Job{
				completedScaleOutInitJob("mariadb", "test", 1, "stale-uid"),
			},
			want: false,
		},
		"all existing tail pvcs must have matching completed init jobs": {
			replicas: 3,
			pvcs: []*corev1.PersistentVolumeClaim{
				scaleOutPVC("mariadb", "test", 1, "first-restored-uid"),
				scaleOutPVC("mariadb", "test", 2, "second-restored-uid"),
			},
			jobs: []*batchv1.Job{
				completedScaleOutInitJob("mariadb", "test", 1, "first-restored-uid"),
			},
			want: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
				t.Fatalf("error adding MariaDB scheme: %v", err)
			}
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatalf("error adding core scheme: %v", err)
			}
			if err := batchv1.AddToScheme(scheme); err != nil {
				t.Fatalf("error adding batch scheme: %v", err)
			}

			mariadb := &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb",
					Namespace: "test",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: tc.replicas,
				},
			}

			objects := []runtime.Object{mariadb}
			for _, pvc := range tc.pvcs {
				objects = append(objects, pvc)
			}
			for _, job := range tc.jobs {
				objects = append(objects, job)
			}
			reconciler := &MariaDBReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(objects...).
					Build(),
			}

			got, err := reconciler.hasCompletedScaleOutInitJobPVCs(context.Background(), mariadb, 1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("unexpected completed init job PVC detection: got %t, want %t", got, tc.want)
			}
		})
	}
}

func TestReconcileScaleOutErrorContinuesAfterCompletedInitJobRestoredPVC(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mariadbv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding MariaDB scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding core scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("error adding batch scheme: %v", err)
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
					Type:    mariadbv1alpha1.ConditionTypeScaledOut,
					Status:  metav1.ConditionFalse,
					Reason:  mariadbv1alpha1.ConditionReasonScaleOutError,
					Message: "Scale out error: storage PVCs already exist",
				},
			},
		},
	}

	reconciler := &MariaDBReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&mariadbv1alpha1.MariaDB{}).
			WithObjects(
				mariadb,
				scaleOutPVC("mariadb", "test", 1, "restored-uid"),
				completedScaleOutInitJob("mariadb", "test", 1, "restored-uid"),
			).
			Build(),
	}

	result, err := reconciler.reconcileScaleOutError(context.Background(), mariadb, 1, logr.Discard())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("expected scale out to continue without requeue, got %v", result)
	}
}

func scaleOutPVC(mariadbName, namespace string, podIndex int, uid string) *corev1.PersistentVolumeClaim {
	mariadb := mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadbName,
			Namespace: namespace,
		},
	}
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PVCKey(builder.StorageVolume, podIndex).Name,
			Namespace: namespace,
			UID:       typesUID(uid),
		},
	}
}

func completedScaleOutInitJob(mariadbName, namespace string, podIndex int, pvcUID string) *batchv1.Job {
	return scaleOutInitJob(mariadbName, namespace, podIndex, pvcUID, true)
}

func scaleOutInitJob(mariadbName, namespace string, podIndex int, pvcUID string, complete bool) *batchv1.Job {
	mariadb := mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadbName,
			Namespace: namespace,
		},
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mariadb.PhysicalBackupInitJobKey(podIndex).Name,
			Namespace: namespace,
			Annotations: map[string]string{
				initJobStoragePVCUIDAnnotation: pvcUID,
			},
		},
	}
	if complete {
		job.Status.Conditions = []batchv1.JobCondition{
			{
				Type:   batchv1.JobComplete,
				Status: corev1.ConditionTrue,
			},
		}
	}
	return job
}

func typesUID(uid string) types.UID {
	return types.UID(uid)
}
