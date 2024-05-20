package builder

import (
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestMariadbImagePullSecrets(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name:      "mariadb-image-pull-secrets",
		Namespace: "test",
	}

	tests := []struct {
		name            string
		mariadb         *mariadbv1alpha1.MariaDB
		wantPullSecrets []corev1.LocalObjectReference
	}{
		{
			name: "No Secrets",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MariaDBSpec{},
			},
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in MariaDB",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{
								Name: "mariadb-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "mariadb-registry",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildMariadbStatefulSet(tt.mariadb, client.ObjectKeyFromObject(tt.mariadb))
			if err != nil {
				t.Fatalf("unexpected error building StatefulSet: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}

func TestMaxScaleImagePullSecrets(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name:      "maxscale-image-pull-secrets",
		Namespace: "test",
	}

	tests := []struct {
		name            string
		maxScale        *mariadbv1alpha1.MaxScale
		wantPullSecrets []corev1.LocalObjectReference
	}{
		{
			name: "No Secrets",
			maxScale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec:       mariadbv1alpha1.MaxScaleSpec{},
			},
			wantPullSecrets: nil,
		},
		{
			name: "Secrets in MaxScale",
			maxScale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{
								Name: "maxscale-registry",
							},
						},
					},
				},
			},
			wantPullSecrets: []corev1.LocalObjectReference{
				{
					Name: "maxscale-registry",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := builder.BuildMaxscaleStatefulSet(tt.maxScale, client.ObjectKeyFromObject(tt.maxScale))
			if err != nil {
				t.Fatalf("unexpected error building StatefulSet: %v", err)
			}
			if !reflect.DeepEqual(tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets) {
				t.Errorf("unexpected ImagePullSecrets, want: %v  got: %v", tt.wantPullSecrets, job.Spec.Template.Spec.ImagePullSecrets)
			}
		})
	}
}

func TestMariaDBStatefulSetMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	key := types.NamespacedName{
		Name: "mariadb-obj",
	}
	tests := []struct {
		name     string
		mariadb  *mariadbv1alpha1.MariaDB
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "inherit meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
		{
			name: "HA",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{},
				Annotations: map[string]string{
					"k8s.mariadb.com/mariadb": "mariadb-obj",
					"k8s.mariadb.com/galera":  "",
				},
			},
		},
		{
			name: "all",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io":       "mariadb",
					"k8s.mariadb.com/mariadb": "mariadb-obj",
					"k8s.mariadb.com/galera":  "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sts, err := builder.BuildMariadbStatefulSet(tt.mariadb, key)
			if err != nil {
				t.Fatalf("unexpected error building MariaDB StatefulSet: %v", err)
			}
			assertObjectMeta(t, &sts.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}

func TestMariaDBUpdateStrategy(t *testing.T) {
	objMeta := metav1.ObjectMeta{
		Name: "mariadb-obj",
	}
	tests := []struct {
		name               string
		mariadb            *mariadbv1alpha1.MariaDB
		wantUpdateStrategy appsv1.StatefulSetUpdateStrategy
	}{
		{
			name: "empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
			},
			wantUpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
		{
			name: "on delete",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Updates: mariadbv1alpha1.Updates{
						Type: mariadbv1alpha1.OnDeleteUpdateType,
					},
				},
			},
			wantUpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.OnDeleteStatefulSetStrategyType,
			},
		},
		{
			name: "rolling update",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Updates: mariadbv1alpha1.Updates{
						Type: mariadbv1alpha1.RollingUpdateUpdateType,
						RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
							MaxUnavailable: ptr.To(intstr.FromInt(1)),
						},
					},
				},
			},
			wantUpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					MaxUnavailable: ptr.To(intstr.FromInt(1)),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stsStategy := mariadbUpdateStrategy(tt.mariadb)
			if !reflect.DeepEqual(tt.wantUpdateStrategy, stsStategy) {
				t.Errorf("expecting mariadbUpdateStrategy returned value to be:\n%v\ngot:\n%v\n", tt.wantUpdateStrategy, stsStategy)
			}
		})
	}
}

func TestMaxScaleStatefulSetMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	objMeta := metav1.ObjectMeta{
		Name: "maxscale-obj",
	}
	key := types.NamespacedName{
		Name: "maxscale-obj",
	}
	tests := []struct {
		name     string
		maxscale *mariadbv1alpha1.MaxScale
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name: "empty",
			maxscale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		},
		{
			name: "inherit meta",
			maxscale: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sts, err := builder.BuildMaxscaleStatefulSet(tt.maxscale, key)
			if err != nil {
				t.Fatalf("unexpected error building MaxScale StatefulSet: %v", err)
			}
			assertObjectMeta(t, &sts.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}
