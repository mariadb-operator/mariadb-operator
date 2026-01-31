package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestInvalidBackupStoragePVC(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "invalid-backup-pvc",
	}
	tests := []struct {
		name    string
		backup  *mariadbv1alpha1.Backup
		wantErr bool
	}{
		{
			name:    "empty",
			backup:  &mariadbv1alpha1.Backup{},
			wantErr: true,
		},
		{
			name: "PVC",
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Storage: mariadbv1alpha1.BackupStorage{
						PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("100Mi"),
								},
							},
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := builder.BuildBackupStoragePVC(
				key,
				tt.backup.Spec.Storage.PersistentVolumeClaim,
				tt.backup.Spec.InheritMetadata,
			)
			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
		})
	}
}

func TestBackupStoragePVCMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "backup-pvc",
	}
	tests := []struct {
		name     string
		backup   *mariadbv1alpha1.Backup
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "PVC",
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Storage: mariadbv1alpha1.BackupStorage{
						PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("100Mi"),
								},
							},
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
						},
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "PVC and inherit meta",
			backup: &mariadbv1alpha1.Backup{
				Spec: mariadbv1alpha1.BackupSpec{
					Storage: mariadbv1alpha1.BackupStorage{
						PersistentVolumeClaim: &mariadbv1alpha1.PersistentVolumeClaimSpec{
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("100Mi"),
								},
							},
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
						},
					},
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvc, err := builder.BuildBackupStoragePVC(
				key,
				tt.backup.Spec.Storage.PersistentVolumeClaim,
				tt.backup.Spec.InheritMetadata,
			)
			if err != nil {
				t.Fatalf("unexpected error building Backup PVC: %v", err)
			}
			assertObjectMeta(t, &pvc.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}

func TestBackupStagingPVCOwnerReference(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name:      "staging-pvc",
		Namespace: "test",
	}
	pvcSpec := &mariadbv1alpha1.PersistentVolumeClaimSpec{
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("500Mi"),
			},
		},
		AccessModes: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},
	}
	meta := &mariadbv1alpha1.Metadata{
		Labels: map[string]string{
			"test-label": "test",
		},
	}
	owner := &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mariadb",
			Namespace: "test",
			UID:       types.UID("test-uid"),
		},
	}

	tests := []struct {
		name         string
		owner        *mariadbv1alpha1.MariaDB
		wantOwnerRef bool
	}{
		{
			name:         "with owner",
			owner:        owner,
			wantOwnerRef: true,
		},
		{
			name:         "without owner",
			owner:        nil,
			wantOwnerRef: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvc, err := builder.BuildStagingPVC(key, pvcSpec, meta, tt.owner)
			assert.NoError(t, err, "unexpected error building Backup Staging PVC")
			assert.NotNil(t, pvc, "expected PVC to be created")

			found := false
			for _, ref := range pvc.OwnerReferences {
				if tt.owner != nil && ref.UID == tt.owner.UID && ref.Name == tt.owner.Name && ref.Kind == "MariaDB" {
					found = true
					assert.True(t, *ref.Controller, "expected Controller to be true")
					break
				}
			}
			assert.Equal(t, tt.wantOwnerRef, found, "unexpected owner reference presence")
		})
	}
}

func TestStoragePVCMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "backup-pvc",
	}
	mariadbObjMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	tests := []struct {
		name     string
		tpl      *mariadbv1alpha1.VolumeClaimTemplate
		mariadb  *mariadbv1alpha1.MariaDB
		wantMeta *mariadbv1alpha1.Metadata
		wantErr  bool
	}{
		{
			name: "no tpl",
			tpl:  nil,
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantErr: true,
		},
		{
			name: "empty",
			tpl:  &mariadbv1alpha1.VolumeClaimTemplate{},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"pvc.k8s.mariadb.com/role":   "storage",
				},
				Annotations: map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "tpl",
			tpl: &mariadbv1alpha1.VolumeClaimTemplate{
				Metadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"database.myorg.io": "mariadb",
					},
					Annotations: map[string]string{
						"database.myorg.io": "mariadb",
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"pvc.k8s.mariadb.com/role":   "storage",
					"database.myorg.io":          "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantErr: false,
		},
		{
			name: "inherit meta",
			tpl:  &mariadbv1alpha1.VolumeClaimTemplate{},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"pvc.k8s.mariadb.com/role":   "storage",
					"database.myorg.io":          "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantErr: false,
		},
		{
			name: "all",
			tpl: &mariadbv1alpha1.VolumeClaimTemplate{
				Metadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"sidecar.istio.io/inject": "false",
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "true",
						},
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"pvc.k8s.mariadb.com/role":   "storage",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "tpl override inherit meta",
			tpl: &mariadbv1alpha1.VolumeClaimTemplate{
				Metadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"sidecar.istio.io/inject": "false",
					},
				},
			},
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: mariadbObjMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "mariadb",
					"app.kubernetes.io/instance": "mariadb-obj",
					"database.myorg.io":          "mariadb",
					"pvc.k8s.mariadb.com/role":   "storage",
					"sidecar.istio.io/inject":    "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvc, err := builder.BuildStoragePVC(key, tt.tpl, tt.mariadb)
			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
			if pvc != nil {
				assertObjectMeta(t, &pvc.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
			}
		})
	}
}

func TestStoragePVCDataSource(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{Name: "snapshot-pvc"}
	tpl := &mariadbv1alpha1.VolumeClaimTemplate{
		PersistentVolumeClaimSpec: mariadbv1alpha1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
		},
	}
	mariadb := &mariadbv1alpha1.MariaDB{}

	tests := []struct {
		name             string
		opts             []PVCOption
		wantDataSource   bool
		wantSnapshotName string
	}{
		{
			name:           "without WithVolumeSnapshotDataSource",
			opts:           []PVCOption{},
			wantDataSource: false,
		},
		{
			name:             "with WithVolumeSnapshotDataSource",
			opts:             []PVCOption{WithVolumeSnapshotDataSource("my-snapshot")},
			wantDataSource:   true,
			wantSnapshotName: "my-snapshot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvc, err := builder.BuildStoragePVC(key, tpl, mariadb, tt.opts...)
			assert.NoError(t, err, "unexpected error building Storage PVC")

			if tt.wantDataSource {
				assert.NotNil(t, pvc.Spec.DataSource, "expected DataSource to be set")
				assert.Equal(t, "VolumeSnapshot", pvc.Spec.DataSource.Kind, "expected DataSource.Kind to be 'VolumeSnapshot'")

				assert.Equal(t, tt.wantSnapshotName, pvc.Spec.DataSource.Name, "expected DataSource.Name to match")
				assert.NotNil(t, pvc.Spec.DataSource.APIGroup, "expected DataSource.APIGroup to be set")
				assert.Equal(
					t,
					"snapshot.storage.k8s.io",
					*pvc.Spec.DataSource.APIGroup,
					"expected DataSource.APIGroup to be 'snapshot.storage.k8s.io'",
				)
			} else {
				assert.Nil(t, pvc.Spec.DataSource, "expected DataSource to be nil")
			}
		})
	}
}
