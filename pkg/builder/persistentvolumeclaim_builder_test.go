package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("InvalidBackupStoragePVC", func() {
	key := types.NamespacedName{
		Name: "invalid-backup-pvc",
	}
	DescribeTable("building Backup storage PVC",
		func(backup *mariadbv1alpha1.Backup, wantErr bool) {
			builder := newDefaultTestBuilder()
			_, err := builder.BuildBackupStoragePVC(
				key,
				backup.Spec.Storage.PersistentVolumeClaim,
				backup.Spec.InheritMetadata,
			)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("empty", &mariadbv1alpha1.Backup{}, true),
		Entry("PVC", &mariadbv1alpha1.Backup{
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
		}, false),
	)
})

var _ = Describe("BackupStoragePVCMeta", func() {
	key := types.NamespacedName{
		Name: "backup-pvc",
	}
	DescribeTable("building Backup storage PVC metadata",
		func(backup *mariadbv1alpha1.Backup, wantMeta *mariadbv1alpha1.Metadata) {
			builder := newDefaultTestBuilder()
			pvc, err := builder.BuildBackupStoragePVC(
				key,
				backup.Spec.Storage.PersistentVolumeClaim,
				backup.Spec.InheritMetadata,
			)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&pvc.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("PVC", &mariadbv1alpha1.Backup{
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
		}, &mariadbv1alpha1.Metadata{
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		}),
		Entry("PVC and inherit meta", &mariadbv1alpha1.Backup{
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
		}, &mariadbv1alpha1.Metadata{
			Labels: map[string]string{
				"database.myorg.io": "mariadb",
			},
			Annotations: map[string]string{
				"database.myorg.io": "mariadb",
			},
		}),
	)
})

var _ = Describe("BackupStagingPVCOwnerReference", func() {
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

	DescribeTable("building Backup staging PVC owner reference",
		func(owner *mariadbv1alpha1.MariaDB, wantOwnerRef bool) {
			builder := newDefaultTestBuilder()
			pvc, err := builder.BuildStagingPVC(key, pvcSpec, meta, owner)
			Expect(err).NotTo(HaveOccurred())
			Expect(pvc).NotTo(BeNil())

			found := false
			for _, ref := range pvc.OwnerReferences {
				if owner != nil && ref.UID == owner.UID && ref.Name == owner.Name && ref.Kind == "MariaDB" {
					found = true
					Expect(*ref.Controller).To(BeTrue())
					break
				}
			}
			Expect(found).To(Equal(wantOwnerRef))
		},
		Entry("with owner", owner, true),
		Entry("without owner", (*mariadbv1alpha1.MariaDB)(nil), false),
	)
})

var _ = Describe("StoragePVCMeta", func() {
	key := types.NamespacedName{
		Name: "backup-pvc",
	}
	mariadbObjMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	DescribeTable("building storage PVC metadata",
		func(tpl *mariadbv1alpha1.VolumeClaimTemplate, mariadb *mariadbv1alpha1.MariaDB, wantMeta *mariadbv1alpha1.Metadata, wantErr bool) {
			builder := newDefaultTestBuilder()
			pvc, err := builder.BuildStoragePVC(key, tpl, mariadb)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if pvc != nil {
				assertObjectMeta(&pvc.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
			}
		},
		Entry("no tpl", nil, &mariadbv1alpha1.MariaDB{
			ObjectMeta: mariadbObjMeta,
		}, &mariadbv1alpha1.Metadata{
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		}, true),
		Entry("empty", &mariadbv1alpha1.VolumeClaimTemplate{}, &mariadbv1alpha1.MariaDB{
			ObjectMeta: mariadbObjMeta,
		}, &mariadbv1alpha1.Metadata{
			Labels: map[string]string{
				"app.kubernetes.io/name":     "mariadb",
				"app.kubernetes.io/instance": "mariadb-obj",
				"pvc.k8s.mariadb.com/role":   "storage",
			},
			Annotations: map[string]string{},
		}, false),
		Entry("tpl", &mariadbv1alpha1.VolumeClaimTemplate{
			Metadata: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		}, &mariadbv1alpha1.MariaDB{
			ObjectMeta: mariadbObjMeta,
		}, &mariadbv1alpha1.Metadata{
			Labels: map[string]string{
				"app.kubernetes.io/name":     "mariadb",
				"app.kubernetes.io/instance": "mariadb-obj",
				"pvc.k8s.mariadb.com/role":   "storage",
				"database.myorg.io":          "mariadb",
			},
			Annotations: map[string]string{
				"database.myorg.io": "mariadb",
			},
		}, false),
		Entry("inherit meta", &mariadbv1alpha1.VolumeClaimTemplate{}, &mariadbv1alpha1.MariaDB{
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
		}, &mariadbv1alpha1.Metadata{
			Labels: map[string]string{
				"app.kubernetes.io/name":     "mariadb",
				"app.kubernetes.io/instance": "mariadb-obj",
				"pvc.k8s.mariadb.com/role":   "storage",
				"database.myorg.io":          "mariadb",
			},
			Annotations: map[string]string{
				"database.myorg.io": "mariadb",
			},
		}, false),
		Entry("all", &mariadbv1alpha1.VolumeClaimTemplate{
			Metadata: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
			},
		}, &mariadbv1alpha1.MariaDB{
			ObjectMeta: mariadbObjMeta,
			Spec: mariadbv1alpha1.MariaDBSpec{
				InheritMetadata: &mariadbv1alpha1.Metadata{
					Labels: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
				},
			},
		}, &mariadbv1alpha1.Metadata{
			Labels: map[string]string{
				"app.kubernetes.io/name":     "mariadb",
				"app.kubernetes.io/instance": "mariadb-obj",
				"pvc.k8s.mariadb.com/role":   "storage",
				"sidecar.istio.io/inject":    "false",
			},
			Annotations: map[string]string{},
		}, false),
		Entry("tpl override inherit meta", &mariadbv1alpha1.VolumeClaimTemplate{
			Metadata: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
			},
		}, &mariadbv1alpha1.MariaDB{
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
		}, &mariadbv1alpha1.Metadata{
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
		}, false),
	)
})

var _ = Describe("StoragePVCDataSource", func() {
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

	DescribeTable("building storage PVC data source",
		func(opts []PVCOption, wantDataSource bool, wantSnapshotName string) {
			builder := newDefaultTestBuilder()
			pvc, err := builder.BuildStoragePVC(key, tpl, mariadb, opts...)
			Expect(err).NotTo(HaveOccurred())

			if wantDataSource {
				Expect(pvc.Spec.DataSource).NotTo(BeNil())
				Expect(pvc.Spec.DataSource.Kind).To(Equal("VolumeSnapshot"))

				Expect(pvc.Spec.DataSource.Name).To(Equal(wantSnapshotName))
				Expect(pvc.Spec.DataSource.APIGroup).NotTo(BeNil())
				Expect(*pvc.Spec.DataSource.APIGroup).To(Equal("snapshot.storage.k8s.io"))
			} else {
				Expect(pvc.Spec.DataSource).To(BeNil())
			}
		},
		Entry("without WithVolumeSnapshotDataSource", []PVCOption{}, false, ""),
		Entry("with WithVolumeSnapshotDataSource", []PVCOption{WithVolumeSnapshotDataSource("my-snapshot")}, true, "my-snapshot"),
	)
})
