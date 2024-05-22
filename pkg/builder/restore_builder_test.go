package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestRestoreMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "restore",
	}
	tests := []struct {
		name            string
		mariadb         *mariadbv1alpha1.MariaDB
		wantRestoreMeta *mariadbv1alpha1.Metadata
		wantPodMeta     *mariadbv1alpha1.Metadata
	}{
		{
			name:    "no meta",
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantRestoreMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "inherit meta",
			mariadb: &mariadbv1alpha1.MariaDB{
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
			wantRestoreMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "pod meta",
			mariadb: &mariadbv1alpha1.MariaDB{
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
			wantRestoreMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "job",
				},
				Annotations: map[string]string{
					"database.myorg.io": "job",
				},
			},
		},
		{
			name: "all",
			mariadb: &mariadbv1alpha1.MariaDB{
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
			wantRestoreMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
			wantPodMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "job",
				},
				Annotations: map[string]string{
					"database.myorg.io": "job",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore, err := builder.BuildRestore(tt.mariadb, key)
			if err != nil {
				t.Fatalf("unexpected error building Restore: %v", err)
			}
			assertObjectMeta(t, &restore.ObjectMeta, tt.wantRestoreMeta.Labels, tt.wantRestoreMeta.Annotations)
			meta := ptr.Deref(restore.Spec.PodMetadata, mariadbv1alpha1.Metadata{})
			assertMeta(t, &meta, tt.wantPodMeta.Labels, tt.wantPodMeta.Annotations)
		})
	}
}

func TestBuildRestore(t *testing.T) {
	builder := newDefaultTestBuilder(t)
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
						Affinity:            corev1.Affinity{},
					},
					Resources: &corev1.ResourceRequirements{
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
	if err != nil {
		t.Errorf("unexpected error building Restore: %v", err)
	}

	if restore.Spec.JobPodTemplate.Affinity == nil {
		t.Error("expected affinity to have been set")
	}
	if restore.Spec.JobContainerTemplate.Resources == nil {
		t.Error("expected resources to have been set")
	}
	if restore.Spec.JobContainerTemplate.Args == nil {
		t.Error("expected args to have been set")
	}
}
