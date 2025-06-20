package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	labelsbuilder "github.com/mariadb-operator/mariadb-operator/pkg/builder/labels"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestInvalidVolumeSnapshot(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{Name: "test-snapshot", Namespace: "test-ns"}
	pvcKey := types.NamespacedName{Name: "test-pvc"}

	tests := []struct {
		name    string
		backup  *mariadbv1alpha1.PhysicalBackup
		wantErr bool
	}{
		{
			name: "VolumeSnapshot is nil returns error",
			backup: &mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "backup-obj",
					Namespace: "test-ns",
					UID:       types.UID("backup-uid"),
				},
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						VolumeSnapshot: nil,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "VolumeSnapshot no error",
			backup: &mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "backup-obj",
					Namespace: "test-ns",
					UID:       types.UID("backup-uid"),
				},
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						VolumeSnapshot: &mariadbv1alpha1.PhysicalBackupVolumeSnapshot{
							VolumeSnapshotClassName: "test-class",
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := builder.BuildVolumeSnapshot(key, tt.backup, pvcKey)
			if tt.wantErr {
				assert.Error(t, err, "expected error")
			} else {
				assert.NoError(t, err, "unexpected error")
			}
		})
	}
}

func TestVolumeSnapshotMetadata(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name:      "test-snapshot",
		Namespace: "test",
	}
	pvcKey := types.NamespacedName{
		Name: "test-pvc",
	}
	objMeta := metav1.ObjectMeta{
		Name:      "backup-obj",
		Namespace: "test",
	}

	tests := []struct {
		name            string
		backup          *mariadbv1alpha1.PhysicalBackup
		wantLabels      map[string]string
		wantAnnotations map[string]string
	}{
		{
			name: "No metadata",
			backup: &mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "backup-obj",
					Namespace: "test-ns",
					UID:       types.UID("backup-uid"),
				},
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					InheritMetadata: nil,
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						VolumeSnapshot: &mariadbv1alpha1.PhysicalBackupVolumeSnapshot{
							VolumeSnapshotClassName: "test-class",
							Metadata:                nil,
						},
					},
				},
			},
			wantLabels: map[string]string{
				labelsbuilder.PhysicalBackupName: "backup-obj",
			},
			wantAnnotations: map[string]string{},
		},
		{
			name: "Only snapshot metadata",
			backup: &mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "backup-obj",
					Namespace: "test-ns",
					UID:       types.UID("backup-uid"),
				},
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					InheritMetadata: nil,
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						VolumeSnapshot: &mariadbv1alpha1.PhysicalBackupVolumeSnapshot{
							VolumeSnapshotClassName: "test-class",
							Metadata: &mariadbv1alpha1.Metadata{
								Labels: map[string]string{
									"snapshot-label": "snapshot-value",
								},
								Annotations: map[string]string{
									"snapshot-annotation": "snapshot-annotation-value",
								},
							},
						},
					},
				},
			},
			wantLabels: map[string]string{
				"snapshot-label":                 "snapshot-value",
				labelsbuilder.PhysicalBackupName: "backup-obj",
			},
			wantAnnotations: map[string]string{
				"snapshot-annotation": "snapshot-annotation-value",
			},
		},
		{
			name: "Only inherit metadata",
			backup: &mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "backup-obj",
					Namespace: "test-ns",
					UID:       types.UID("backup-uid"),
				},
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"custom-label": "custom-value",
						},
						Annotations: map[string]string{
							"custom-annotation": "custom-annotation-value",
						},
					},
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						VolumeSnapshot: &mariadbv1alpha1.PhysicalBackupVolumeSnapshot{
							VolumeSnapshotClassName: "test-class",
							Metadata:                nil,
						},
					},
				},
			},
			wantLabels: map[string]string{
				"custom-label":                   "custom-value",
				labelsbuilder.PhysicalBackupName: "backup-obj",
			},
			wantAnnotations: map[string]string{
				"custom-annotation": "custom-annotation-value",
			},
		},
		{
			name: "Inherit and snapshot metadata merged",
			backup: &mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"custom-label": "custom-value",
						},
						Annotations: map[string]string{
							"custom-annotation": "custom-annotation-value",
						},
					},
					Storage: mariadbv1alpha1.PhysicalBackupStorage{
						VolumeSnapshot: &mariadbv1alpha1.PhysicalBackupVolumeSnapshot{
							VolumeSnapshotClassName: "test-class",
							Metadata: &mariadbv1alpha1.Metadata{
								Labels: map[string]string{
									"snapshot-label": "snapshot-value",
								},
								Annotations: map[string]string{
									"snapshot-annotation": "snapshot-annotation-value",
								},
							},
						},
					},
				},
			},
			wantLabels: map[string]string{
				"custom-label":                   "custom-value",
				"snapshot-label":                 "snapshot-value",
				labelsbuilder.PhysicalBackupName: "backup-obj",
			},
			wantAnnotations: map[string]string{
				"custom-annotation":   "custom-annotation-value",
				"snapshot-annotation": "snapshot-annotation-value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot, err := builder.BuildVolumeSnapshot(key, tt.backup, pvcKey)
			assert.NoError(t, err, "unexpected error building VolumeSnapshot")
			assert.NotNil(t, snapshot, "expected snapshot to be created")
			assert.Equal(t, key.Name, snapshot.Name)
			assert.Equal(t, key.Namespace, snapshot.Namespace)
			assert.Equal(t, "test-class", *snapshot.Spec.VolumeSnapshotClassName)
			assert.NotNil(t, snapshot.Spec.Source.PersistentVolumeClaimName)
			assert.Equal(t, pvcKey.Name, *snapshot.Spec.Source.PersistentVolumeClaimName)

			for k, v := range tt.wantLabels {
				assert.Equal(t, v, snapshot.Labels[k], "expected label %s to be %s", k, v)
			}
			for k, v := range tt.wantAnnotations {
				assert.Equal(t, v, snapshot.Annotations[k], "expected annotation %s to be %s", k, v)
			}
		})
	}
}
