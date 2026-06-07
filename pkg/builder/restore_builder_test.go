package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var _ = Describe("RestoreMeta", func() {
	builder := newDefaultTestBuilder()
	key := types.NamespacedName{
		Name: "restore",
	}

	DescribeTable(
		"should build Restore meta",
		func(
			mariadb *mariadbv1alpha1.MariaDB,
			wantRestoreMeta *mariadbv1alpha1.Metadata,
			wantPodMeta *mariadbv1alpha1.Metadata,
		) {
			restore, err := builder.BuildRestore(mariadb, key)
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&restore.ObjectMeta, wantRestoreMeta.Labels, wantRestoreMeta.Annotations)
			meta := ptr.Deref(restore.Spec.PodMetadata, mariadbv1alpha1.Metadata{})
			assertMeta(&meta, wantPodMeta.Labels, wantPodMeta.Annotations)
		},
		Entry(
			"no meta",
			&mariadbv1alpha1.MariaDB{},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry(
			"inherit meta",
			&mariadbv1alpha1.MariaDB{
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
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry(
			"pod meta",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
						RestoreJob: &mariadbv1alpha1.Job{
							Metadata: &mariadbv1alpha1.Metadata{
								Labels: map[string]string{
									"database.myorg.io": "job",
								},
								Annotations: map[string]string{
									"database.myorg.io": "job",
								},
							},
						},
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "job",
				},
				Annotations: map[string]string{
					"database.myorg.io": "job",
				},
			},
		),
		Entry(
			"all",
			&mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
						RestoreJob: &mariadbv1alpha1.Job{
							Metadata: &mariadbv1alpha1.Metadata{
								Labels: map[string]string{
									"database.myorg.io": "job",
								},
								Annotations: map[string]string{
									"database.myorg.io": "job",
								},
							},
						},
					},
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "job",
				},
				Annotations: map[string]string{
					"database.myorg.io": "job",
				},
			},
		),
	)
})

var _ = Describe("BuildRestore", func() {
	It("should build Restore", func() {
		builder := newDefaultTestBuilder()
		mariadb := &mariadbv1alpha1.MariaDB{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-restore-builder",
			},
			Spec: mariadbv1alpha1.MariaDBSpec{
				Storage: mariadbv1alpha1.Storage{
					Size: ptr.To(resource.MustParse("300Mi")),
				},
				BootstrapFrom: &mariadbv1alpha1.BootstrapFrom{
					RestoreJob: &mariadbv1alpha1.Job{
						Affinity: &mariadbv1alpha1.AffinityConfig{
							AntiAffinityEnabled: ptr.To(true),
							Affinity:            mariadbv1alpha1.Affinity{},
						},
						NodeSelector: map[string]string{
							"kubernetes.io/hostname": "compute-0",
						},
						Tolerations: []corev1.Toleration{},
						Resources: &mariadbv1alpha1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"cpu": resource.MustParse("100m"),
							},
						},
						Args: []string{"--verbose"},
					},
				},
			},
		}
		key := types.NamespacedName{
			Name: "test-restore",
		}

		restore, err := builder.BuildRestore(mariadb, key)
		Expect(err).NotTo(HaveOccurred())

		Expect(restore.Spec.Affinity).NotTo(BeNil())
		Expect(restore.Spec.NodeSelector).ToNot(BeEmpty())
		Expect(restore.Spec.Tolerations).NotTo(BeNil())
		Expect(restore.Spec.Resources).NotTo(BeNil())
		Expect(restore.Spec.Args).NotTo(BeNil())
	})
})
