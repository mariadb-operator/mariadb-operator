package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("InvalidVolumeSnapshot", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{Name: "test-snapshot", Namespace: "test-ns"}
	pvcKey := types.NamespacedName{Name: "test-pvc"}

	DescribeTable("BuildVolumeSnapshot",
		func(backup *mariadbv1alpha1.PhysicalBackup, wantErr bool) {
			_, err := builder.BuildVolumeSnapshot(key, backup, pvcKey, nil)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("VolumeSnapshot is nil returns error",
			&mariadbv1alpha1.PhysicalBackup{
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
			true,
		),
		Entry("VolumeSnapshot no error",
			&mariadbv1alpha1.PhysicalBackup{
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
			false,
		),
	)
})

var _ = Describe("VolumeSnapshotMetadata", func() {
	builder := newDefaultTestBuilder()
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

	DescribeTable("BuildVolumeSnapshot",
		func(backup *mariadbv1alpha1.PhysicalBackup, meta *mariadbv1alpha1.Metadata,
			wantLabels, wantAnnotations map[string]string) {
			snapshot, err := builder.BuildVolumeSnapshot(key, backup, pvcKey, meta)
			Expect(err).NotTo(HaveOccurred())
			Expect(snapshot).NotTo(BeNil())
			Expect(snapshot.Name).To(Equal(key.Name))
			Expect(snapshot.Namespace).To(Equal(key.Namespace))
			Expect(*snapshot.Spec.VolumeSnapshotClassName).To(Equal("test-class"))
			Expect(snapshot.Spec.Source.PersistentVolumeClaimName).NotTo(BeNil())
			Expect(*snapshot.Spec.Source.PersistentVolumeClaimName).To(Equal(pvcKey.Name))

			for k, v := range wantLabels {
				Expect(snapshot.Labels[k]).To(Equal(v))
			}
			for k, v := range wantAnnotations {
				Expect(snapshot.Annotations[k]).To(Equal(v))
			}
		},
		Entry("No metadata",
			&mariadbv1alpha1.PhysicalBackup{
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
			nil,
			map[string]string{
				metadata.PhysicalBackupNameLabel: "backup-obj",
			},
			map[string]string{},
		),
		Entry("Only snapshot metadata",
			&mariadbv1alpha1.PhysicalBackup{
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
			nil,
			map[string]string{
				"snapshot-label":                 "snapshot-value",
				metadata.PhysicalBackupNameLabel: "backup-obj",
			},
			map[string]string{
				"snapshot-annotation": "snapshot-annotation-value",
			},
		),
		Entry("Only inherit metadata",
			&mariadbv1alpha1.PhysicalBackup{
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
			nil,
			map[string]string{
				"custom-label":                   "custom-value",
				metadata.PhysicalBackupNameLabel: "backup-obj",
			},
			map[string]string{
				"custom-annotation": "custom-annotation-value",
			},
		),
		Entry("Inherit and snapshot metadata merged",
			&mariadbv1alpha1.PhysicalBackup{
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
			nil,
			map[string]string{
				"custom-label":                   "custom-value",
				"snapshot-label":                 "snapshot-value",
				metadata.PhysicalBackupNameLabel: "backup-obj",
			},
			map[string]string{
				"custom-annotation":   "custom-annotation-value",
				"snapshot-annotation": "snapshot-annotation-value",
			},
		),
		Entry("Metadata by argument",
			&mariadbv1alpha1.PhysicalBackup{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.PhysicalBackupSpec{
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
			&mariadbv1alpha1.Metadata{
				Annotations: map[string]string{
					metadata.GtidAnnotation: "0-10-1897",
				},
			},
			map[string]string{
				metadata.PhysicalBackupNameLabel: "backup-obj",
			},
			map[string]string{
				metadata.GtidAnnotation: "0-10-1897",
			},
		),
	)
})
