package builder

import (
	"reflect"
	"slices"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	builderpki "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/pki"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/datastructures"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/discovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func TestMariadbStartupProbe(t *testing.T) {
	tests := []struct {
		name      string
		mariadb   *mariadbv1alpha1.MariaDB
		wantProbe *corev1.Probe
	}{
		{
			name:    "MariaDB",
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB with thresholds",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						StartupProbe: &mariadbv1alpha1.Probe{
							FailureThreshold: 10,
							TimeoutSeconds:   5,
							PeriodSeconds:    10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
				FailureThreshold:    10,
			},
		},
		{
			name: "MariaDB custom",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						StartupProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								Exec: &mariadbv1alpha1.ExecAction{
									Command: []string{
										"bash",
										"-c",
										"mysqladmin ping -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
									},
								},
							},
							FailureThreshold: 10,
							TimeoutSeconds:   10,
							PeriodSeconds:    10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mysqladmin ping -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				FailureThreshold: 10,
				TimeoutSeconds:   10,
				PeriodSeconds:    10,
			},
		},
		{
			name: "MariaDB replication",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5555,
							},
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB replication with thresholds",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5555,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						StartupProbe: &mariadbv1alpha1.Probe{
							FailureThreshold: 10,
							TimeoutSeconds:   10,
							PeriodSeconds:    10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 20,
				FailureThreshold:    10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB replication custom",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5555,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						StartupProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								HTTPGet: &mariadbv1alpha1.HTTPGetAction{
									Path: "/liveness-custom",
									Port: intstr.FromInt(5555),
								},
							},
							FailureThreshold: 10,
							TimeoutSeconds:   10,
							PeriodSeconds:    10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 20,
				FailureThreshold:    10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB replication with standalone probe",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							StandaloneProbes: ptr.To(true),
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB Galera",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5555,
							},
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB Galera with thresholds",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5555,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						StartupProbe: &mariadbv1alpha1.Probe{
							FailureThreshold: 10,
							TimeoutSeconds:   10,
							PeriodSeconds:    10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 20,
				FailureThreshold:    10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB Galera custom",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5555,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						StartupProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								HTTPGet: &mariadbv1alpha1.HTTPGetAction{
									Path: "/liveness-custom",
									Port: intstr.FromInt(5555),
								},
							},
							FailureThreshold: 10,
							TimeoutSeconds:   10,
							PeriodSeconds:    10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 20,
				FailureThreshold:    10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe, err := mariadbStartupProbe(tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.wantProbe, probe); diff != "" {
				t.Errorf("unexpected probe (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMariadbLivenessProbe(t *testing.T) {
	tests := []struct {
		name      string
		mariadb   *mariadbv1alpha1.MariaDB
		wantProbe *corev1.Probe
	}{
		{
			name:    "MariaDB",
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB with thresholds",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &mariadbv1alpha1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB custom",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								Exec: &mariadbv1alpha1.ExecAction{
									Command: []string{
										"bash",
										"-c",
										"mysqladmin ping -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
									},
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mysqladmin ping -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB replication",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB replication with thresholds",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &mariadbv1alpha1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB replication custom",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								HTTPGet: &mariadbv1alpha1.HTTPGetAction{
									Path: "/liveness-custom",
									Port: intstr.FromInt(5566),
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB replication with standalone probe",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							StandaloneProbes: ptr.To(true),
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB Galera",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB Galera with thresholds",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &mariadbv1alpha1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB Galera custom",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								HTTPGet: &mariadbv1alpha1.HTTPGetAction{
									Path: "/liveness-custom",
									Port: intstr.FromInt(5566),
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/liveness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe, err := mariadbLivenessProbe(tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.wantProbe, probe); diff != "" {
				t.Errorf("unexpected probe (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMariadbReadinessProbe(t *testing.T) {
	tests := []struct {
		name      string
		mariadb   *mariadbv1alpha1.MariaDB
		wantProbe *corev1.Probe
	}{
		{
			name:    "MariaDB empty",
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB with thresholds",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &mariadbv1alpha1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB custom",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								Exec: &mariadbv1alpha1.ExecAction{
									Command: []string{
										"bash",
										"-c",
										"mysqladmin ping -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
									},
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"bash",
							"-c",
							"mysqladmin ping -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB replication",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB replication with thresholds",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &mariadbv1alpha1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB replication custom",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								HTTPGet: &mariadbv1alpha1.HTTPGetAction{
									Path: "/readiness-custom",
									Port: intstr.FromInt(5566),
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB replication with ignored standalone probe",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							StandaloneProbes: ptr.To(true),
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB Galera",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB Galera with thresholds",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &mariadbv1alpha1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MariaDB Galera custom",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.Agent{
								ProbePort: 5566,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &mariadbv1alpha1.Probe{
							ProbeHandler: mariadbv1alpha1.ProbeHandler{
								HTTPGet: &mariadbv1alpha1.HTTPGetAction{
									Path: "/readiness-custom",
									Port: intstr.FromInt(5566),
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5566),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe, err := mariadbReadinessProbe(tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.wantProbe, probe); diff != "" {
				t.Errorf("unexpected probe (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMaxScaleProbe(t *testing.T) {
	tests := []struct {
		name      string
		maxScale  *mariadbv1alpha1.MaxScale
		probe     *mariadbv1alpha1.Probe
		wantProbe *corev1.Probe
	}{
		{
			name: "MaxScale empty",
			maxScale: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
				},
			},
			probe: &mariadbv1alpha1.Probe{},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(8989),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MaxScale partial",
			maxScale: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
				},
			},
			probe: &mariadbv1alpha1.Probe{
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(8989),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MaxScale full",
			maxScale: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
				},
			},
			probe: &mariadbv1alpha1.Probe{
				ProbeHandler: mariadbv1alpha1.ProbeHandler{
					TCPSocket: &mariadbv1alpha1.TCPSocketAction{
						Host: "custom",
						Port: intstr.FromInt(8989),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Host: "custom",
						Port: intstr.FromInt(8989),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
		{
			name: "MaxScale Probe with Failure Threshold",
			maxScale: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Admin: mariadbv1alpha1.MaxScaleAdmin{
						Port: 8989,
					},
				},
			},
			probe: &mariadbv1alpha1.Probe{
				ProbeHandler: mariadbv1alpha1.ProbeHandler{
					HTTPGet: &mariadbv1alpha1.HTTPGetAction{
						Path: "/custom",
						Port: intstr.FromInt(8989),
					},
				},
				FailureThreshold:    10,
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/custom",
						Port: intstr.FromInt(8989),
					},
				},
				FailureThreshold:    10,
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := maxscaleProbe(tt.maxScale, tt.probe)
			if diff := cmp.Diff(tt.wantProbe, probe); diff != "" {
				t.Errorf("unexpected probe (-want +got):\n%s", diff)
			}
		})
	}
}

func TestContainerSecurityContext(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	tpl := &mariadbv1alpha1.ContainerTemplate{}

	container, err := builder.buildContainerWithTemplate("mariadb:10.6", corev1.PullIfNotPresent, tpl)
	if err != nil {
		t.Fatalf("unexpected error building container: %v", err)
	}
	if container.SecurityContext != nil {
		t.Error("expected SecurityContext to be nil")
	}

	tpl = &mariadbv1alpha1.ContainerTemplate{
		SecurityContext: &mariadbv1alpha1.SecurityContext{
			RunAsUser: ptr.To(mysqlUser),
		},
	}
	container, err = builder.buildContainerWithTemplate("mariadb:10.6", corev1.PullIfNotPresent, tpl)
	if err != nil {
		t.Fatalf("unexpected error building container: %v", err)
	}
	if container.SecurityContext == nil {
		t.Error("expected SecurityContext not to be nil")
	}

	resource := &metav1.APIResourceList{
		GroupVersion: "security.openshift.io/v1",
		APIResources: []metav1.APIResource{
			{
				Name: "securitycontextconstraints",
			},
		},
	}
	discovery, err := discovery.NewFakeDiscovery(resource)
	if err != nil {
		t.Fatalf("unexpected error getting discovery: %v", err)
	}
	builder = newTestBuilder(discovery)

	container, err = builder.buildContainerWithTemplate("mariadb:10.6", corev1.PullIfNotPresent, tpl)
	if err != nil {
		t.Fatalf("unexpected error building container: %v", err)
	}
	if container.SecurityContext != nil {
		t.Error("expected SecurityContext to be nil")
	}
}

func TestContainerLifecycle(t *testing.T) {
	builder := newDefaultTestBuilder(t)

	// Test without lifecycle
	tpl := &mariadbv1alpha1.ContainerTemplate{}
	container, err := builder.buildContainerWithTemplate("mariadb:10.6", corev1.PullIfNotPresent, tpl)
	if err != nil {
		t.Fatalf("unexpected error building container: %v", err)
	}
	if container.Lifecycle != nil {
		t.Error("expected Lifecycle to be nil")
	}

	// Test with lifecycle
	tpl = &mariadbv1alpha1.ContainerTemplate{
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"/bin/sh", "-c", "sleep 10"},
				},
			},
			PostStart: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"/bin/sh", "-c", "echo started"},
				},
			},
		},
	}
	container, err = builder.buildContainerWithTemplate("mariadb:10.6", corev1.PullIfNotPresent, tpl)
	if err != nil {
		t.Fatalf("unexpected error building container: %v", err)
	}
	if container.Lifecycle == nil {
		t.Fatal("expected Lifecycle not to be nil")
	}
	if container.Lifecycle.PreStop == nil {
		t.Error("expected PreStop not to be nil")
	}
	if container.Lifecycle.PostStart == nil {
		t.Error("expected PostStart not to be nil")
	}
	if !reflect.DeepEqual(container.Lifecycle.PreStop.Exec.Command, []string{"/bin/sh", "-c", "sleep 10"}) {
		t.Errorf("unexpected PreStop command: %v", container.Lifecycle.PreStop.Exec.Command)
	}
	if !reflect.DeepEqual(container.Lifecycle.PostStart.Exec.Command, []string{"/bin/sh", "-c", "echo started"}) {
		t.Errorf("unexpected PostStart command: %v", container.Lifecycle.PostStart.Exec.Command)
	}
}

func TestMariadbEnv(t *testing.T) {
	tests := []struct {
		name           string
		mariadb        *mariadbv1alpha1.MariaDB
		wantEnv        []corev1.EnvVar
		setClusterName bool
	}{
		{
			name:    "MariaDB empty",
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantEnv: defaultEnv(nil),
		},
		{
			name:    "MariaDB cluster name",
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantEnv: defaultEnv([]corev1.EnvVar{
				{
					Name:  "CLUSTER_NAME",
					Value: "example.com",
				},
			}),
			setClusterName: true,
		},
		{
			name: "MariaDB tcp port",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 12345,
				},
			},
			wantEnv: defaultEnv([]corev1.EnvVar{
				{
					Name:  "MYSQL_TCP_PORT",
					Value: strconv.Itoa(12345),
				},
			}),
		},
		{
			name: "MariaDB name",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example",
				},
			},
			wantEnv: defaultEnv([]corev1.EnvVar{
				{
					Name:  "MARIADB_NAME",
					Value: "example",
				},
			}),
		},
		{
			name: "MariaDB root empty password",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootEmptyPassword: ptr.To(true),
				},
			},
			wantEnv: defaultEnv([]corev1.EnvVar{
				{
					Name:  "MARIADB_ALLOW_EMPTY_ROOT_PASSWORD",
					Value: "yes",
				},
			}),
		},
		{
			name: "MariaDB timeZone",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TimeZone: ptr.To("UTC"),
				},
			},
			wantEnv: removeEnv(defaultEnv(nil), "MYSQL_INITDB_SKIP_TZINFO"),
		},
		{
			name: "MariaDB TLS",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					TLS: &mariadbv1alpha1.TLS{
						Enabled: true,
					},
				},
			},
			wantEnv: append(defaultEnv(nil),
				[]corev1.EnvVar{
					{
						Name:  "TLS_ENABLED",
						Value: strconv.FormatBool(true),
					},
					{
						Name:  "TLS_CA_CERT_PATH",
						Value: builderpki.CACertPath,
					},
					{
						Name:  "TLS_SERVER_CERT_PATH",
						Value: builderpki.ServerCertPath,
					},
					{
						Name:  "TLS_SERVER_KEY_PATH",
						Value: builderpki.ServerKeyPath,
					},
					{
						Name:  "TLS_CLIENT_CERT_PATH",
						Value: builderpki.ClientCertPath,
					},
					{
						Name:  "TLS_CLIENT_KEY_PATH",
						Value: builderpki.ClientKeyPath,
					},
				}...),
		},
		{
			name: "MariaDB replication",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mariadb-repl",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							GtidStrictMode:     ptr.To(true),
							SemiSyncEnabled:    ptr.To(true),
							SemiSyncAckTimeout: &metav1.Duration{Duration: 10 * time.Second},
							SemiSyncWaitPoint:  ptr.To(mariadbv1alpha1.WaitPointAfterCommit),
						},
					},
				},
			},
			wantEnv: append(
				defaultEnv([]corev1.EnvVar{
					{
						Name:  "MARIADB_NAME",
						Value: "mariadb-repl",
					},
				}),
				[]corev1.EnvVar{
					{
						Name:  "MARIADB_REPL_ENABLED",
						Value: strconv.FormatBool(true),
					},
					{
						Name:  "MARIADB_REPL_GTID_STRICT_MODE",
						Value: strconv.FormatBool(true),
					},
					{
						Name:  "MARIADB_REPL_SEMI_SYNC_ENABLED",
						Value: strconv.FormatBool(true),
					},
					{
						Name:  "MARIADB_REPL_SEMI_SYNC_MASTER_TIMEOUT",
						Value: "10000",
					},
					{
						Name:  "MARIADB_REPL_SEMI_SYNC_MASTER_WAIT_POINT",
						Value: "AFTER_COMMIT",
					},
				}...),
		},
		{
			name: "MariaDB Galera TLS",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mariadb-galera",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
					},
					TLS: &mariadbv1alpha1.TLS{
						Enabled: true,
					},
				},
			},
			wantEnv: append(
				defaultEnv([]corev1.EnvVar{
					{
						Name:  "MARIADB_NAME",
						Value: "mariadb-galera",
					},
				}),
				[]corev1.EnvVar{
					{
						Name:  "TLS_ENABLED",
						Value: strconv.FormatBool(true),
					},
					{
						Name:  "TLS_CA_CERT_PATH",
						Value: builderpki.CACertPath,
					},
					{
						Name:  "TLS_SERVER_CERT_PATH",
						Value: builderpki.ServerCertPath,
					},
					{
						Name:  "TLS_SERVER_KEY_PATH",
						Value: builderpki.ServerKeyPath,
					},
					{
						Name:  "TLS_CLIENT_CERT_PATH",
						Value: builderpki.ClientCertPath,
					},
					{
						Name:  "TLS_CLIENT_KEY_PATH",
						Value: builderpki.ClientKeyPath,
					},
					{
						Name:  "WSREP_SST_OPT_REMOTE_AUTH",
						Value: "mariadb-galera-client:",
					},
				}...),
		},
		{
			name: "MariaDB env append",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Env: []mariadbv1alpha1.EnvVar{
							{
								Name:  "FOO_BAR_BAZ",
								Value: "LOREM_IPSUM_DOLOR",
							},
						},
					},
				},
			},
			wantEnv: append(defaultEnv(nil), corev1.EnvVar{
				Name:  "FOO_BAR_BAZ",
				Value: "LOREM_IPSUM_DOLOR",
			}),
		},
		{
			name: "MariaDB env override",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Env: []mariadbv1alpha1.EnvVar{
							{
								Name:  "MYSQL_TCP_PORT",
								Value: strconv.Itoa(12345),
							},
							{
								Name:  "MARIADB_ROOT_HOST",
								Value: "1.2.3.4",
							},
							{
								Name:  "CLUSTER_NAME",
								Value: "foo.bar",
							},
							{
								Name:  "POD_NAME",
								Value: "foo",
							},
							{
								Name:  "POD_NAMESPACE",
								Value: "foo",
							},
							{
								Name:  "POD_IP",
								Value: "1.2.3.4",
							},
							{
								Name:  "MARIADB_NAME",
								Value: "foo",
							},
							{
								Name:  "MARIADB_ROOT_PASSWORD",
								Value: "foo",
							},
							{
								Name:  "MYSQL_INITDB_SKIP_TZINFO",
								Value: "0",
							},
						},
					},
				},
			},
			wantEnv: []corev1.EnvVar{
				{
					Name:  "MYSQL_TCP_PORT",
					Value: strconv.Itoa(12345),
				},
				{
					Name:  "MARIADB_ROOT_HOST",
					Value: "1.2.3.4",
				},
				{
					Name:  "CLUSTER_NAME",
					Value: "foo.bar",
				},
				{
					Name:  "POD_NAME",
					Value: "foo",
				},
				{
					Name:  "POD_NAMESPACE",
					Value: "foo",
				},
				{
					Name:  "POD_IP",
					Value: "1.2.3.4",
				},
				{
					Name:  "MARIADB_NAME",
					Value: "foo",
				},
				{
					Name:  "MARIADB_ROOT_PASSWORD",
					Value: "foo",
				},
				{
					Name:  "MYSQL_INITDB_SKIP_TZINFO",
					Value: "0",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setClusterName {
				t.Setenv("CLUSTER_NAME", "example.com")
			}
			env, err := mariadbEnv(tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			sortedWantEnv := sortEnvVars(tt.wantEnv)
			sortedEnv := sortEnvVars(env)

			if diff := cmp.Diff(sortedWantEnv, sortedEnv); diff != "" {
				t.Errorf("unexpected env (-want +got):\n%s", diff)
			}
		})
	}
}

func TestS3Env(t *testing.T) {
	tests := []struct {
		name        string
		s3          *mariadbv1alpha1.S3
		expectedEnv []string
	}{
		{
			name:        "nil S3",
			s3:          nil,
			expectedEnv: nil,
		},
		{
			name: "S3 with access key only",
			s3: &mariadbv1alpha1.S3{
				Bucket:   "test-bucket",
				Endpoint: "s3.amazonaws.com",
				AccessKeyIdSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "access-key-id",
				},
				SecretAccessKeySecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "secret-access-key",
				},
			},
			expectedEnv: []string{S3AccessKeyId, S3SecretAccessKey},
		},
		{
			name: "S3 with session token",
			s3: &mariadbv1alpha1.S3{
				Bucket:   "test-bucket",
				Endpoint: "s3.amazonaws.com",
				AccessKeyIdSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "access-key-id",
				},
				SecretAccessKeySecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "secret-access-key",
				},
				SessionTokenSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "session-token",
				},
			},
			expectedEnv: []string{S3AccessKeyId, S3SecretAccessKey, S3SessionTokenKey},
		},
		{
			name: "S3 with SSE-C",
			s3: &mariadbv1alpha1.S3{
				Bucket:   "test-bucket",
				Endpoint: "s3.amazonaws.com",
				AccessKeyIdSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "access-key-id",
				},
				SecretAccessKeySecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "secret-access-key",
				},
				SSEC: &mariadbv1alpha1.SSECConfig{
					CustomerKeySecretKeyRef: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: "ssec-key",
						},
						Key: "customer-key",
					},
				},
			},
			expectedEnv: []string{S3AccessKeyId, S3SecretAccessKey, S3SSECCustomerKey},
		},
		{
			name: "S3 with all options",
			s3: &mariadbv1alpha1.S3{
				Bucket:   "test-bucket",
				Endpoint: "s3.amazonaws.com",
				AccessKeyIdSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "access-key-id",
				},
				SecretAccessKeySecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "secret-access-key",
				},
				SessionTokenSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
					LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
						Name: "s3-credentials",
					},
					Key: "session-token",
				},
				SSEC: &mariadbv1alpha1.SSECConfig{
					CustomerKeySecretKeyRef: mariadbv1alpha1.SecretKeySelector{
						LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
							Name: "ssec-key",
						},
						Key: "customer-key",
					},
				},
			},
			expectedEnv: []string{S3AccessKeyId, S3SecretAccessKey, S3SessionTokenKey, S3SSECCustomerKey},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := s3Env(tt.s3)

			if tt.expectedEnv == nil {
				if env != nil {
					t.Errorf("expected nil env, got: %v", env)
				}
				return
			}

			if len(env) != len(tt.expectedEnv) {
				t.Errorf("expected %d env vars, got: %d", len(tt.expectedEnv), len(env))
				return
			}

			for _, expectedName := range tt.expectedEnv {
				found := slices.ContainsFunc(env, func(e corev1.EnvVar) bool {
					return e.Name == expectedName
				})
				if !found {
					t.Errorf("expected env var %s not found in %v", expectedName, env)
				}
			}
		})
	}
}

func TestContainerArgs(t *testing.T) {
	tests := []struct {
		name     string
		mariadb  *mariadbv1alpha1.MariaDB
		wantArgs []string
	}{
		{
			name:     "MariaDB args empty",
			mariadb:  &mariadbv1alpha1.MariaDB{},
			wantArgs: nil,
		},
		{
			name: "MariaDB args verbose",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Args: []string{"--verbose"},
					},
				},
			},
			wantArgs: []string{
				"--verbose",
			},
		},
		{
			name: "MariaDB args verbose",
			mariadb: &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mariadb-test",
				},
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Args: []string{"--verbose"},
					},
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
					},
				},
			},
			wantArgs: []string{
				"--verbose",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := mariadbArgs(tt.mariadb)
			if !reflect.DeepEqual(tt.wantArgs, args) {
				t.Errorf("unexpected result:\nexpected:\n%s\ngot:\n%s\n", tt.wantArgs, args)
			}
		})
	}
}

func TestMariadbContainers(t *testing.T) {
	tests := []struct {
		name                string
		mariadb             *mariadbv1alpha1.MariaDB
		wantName            string
		wantEnvKeys         []string
		wantVolumeMountKeys []string
	}{
		{
			name: "Without sidecar container name",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						SidecarContainers: []mariadbv1alpha1.Container{
							{
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep infinity",
								},
							},
						},
					},
				},
			},
			wantName:            "sidecar-0",
			wantEnvKeys:         nil,
			wantVolumeMountKeys: nil,
		},
		{
			name: "With sidecar container name",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						SidecarContainers: []mariadbv1alpha1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep infinity",
								},
							},
						},
					},
				},
			},
			wantName:            "busybox",
			wantEnvKeys:         nil,
			wantVolumeMountKeys: nil,
		},
		{
			name: "With env",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					PodTemplate: mariadbv1alpha1.PodTemplate{
						SidecarContainers: []mariadbv1alpha1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep 1",
								},
								Env: []mariadbv1alpha1.EnvVar{
									{
										Name:  "TEST",
										Value: "TEST",
									},
									{
										Name:  "FOO",
										Value: "FOO",
									},
									{
										Name:  "BAR",
										Value: "BAR",
									},
								},
							},
						},
					},
				},
			},
			wantName:            "busybox",
			wantEnvKeys:         []string{"TEST", "FOO", "BAR"},
			wantVolumeMountKeys: nil,
		},
		{
			name: "With volumeMount",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					PodTemplate: mariadbv1alpha1.PodTemplate{
						SidecarContainers: []mariadbv1alpha1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep 1",
								},
								VolumeMounts: []mariadbv1alpha1.VolumeMount{
									{
										Name:      "TEST",
										MountPath: "/test",
									},
									{
										Name:      "FOO",
										MountPath: "/foo",
									},
									{
										Name:      "BAR",
										MountPath: "/bar",
									},
								},
							},
						},
					},
				},
			},
			wantName:            "busybox",
			wantEnvKeys:         nil,
			wantVolumeMountKeys: []string{"TEST", "FOO", "BAR"},
		},
	}

	builder := newDefaultTestBuilder(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			containers, err := builder.mariadbContainers(tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error building containers: %v", err)
			}

			container := containers[1]

			if container.Name != tt.wantName {
				t.Errorf("unexpected result:\nexpected:\n%s\ngot:\n%s\n", tt.wantName, containers[1].Name)
			}
			if tt.wantEnvKeys != nil {
				idx := datastructures.NewIndex(container.Env, func(env corev1.EnvVar) string {
					return env.Name
				})
				if !datastructures.AllExists(idx, tt.wantEnvKeys...) {
					t.Errorf("expected env keys \"%v\" to exist", tt.wantEnvKeys)
				}
			}
			if tt.wantVolumeMountKeys != nil {
				idx := datastructures.NewIndex(container.VolumeMounts, func(volumeMount corev1.VolumeMount) string {
					return volumeMount.Name
				})
				if !datastructures.AllExists(idx, tt.wantVolumeMountKeys...) {
					t.Errorf("expected volumeMount keys \"%s\" to exist", tt.wantVolumeMountKeys)
				}
			}
		})
	}
}

func TestMariadbInitContainers(t *testing.T) {
	tests := []struct {
		name                string
		mariadb             *mariadbv1alpha1.MariaDB
		wantName            string
		wantEnvKeys         []string
		wantVolumeMountKeys []string
	}{
		{
			name: "Without container name",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						InitContainers: []mariadbv1alpha1.Container{
							{
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep 1",
								},
							},
						},
					},
				},
			},
			wantName:            "init-0",
			wantEnvKeys:         nil,
			wantVolumeMountKeys: nil,
		},
		{
			name: "With container name",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					PodTemplate: mariadbv1alpha1.PodTemplate{
						InitContainers: []mariadbv1alpha1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep 1",
								},
							},
						},
					},
				},
			},
			wantName:            "busybox",
			wantEnvKeys:         nil,
			wantVolumeMountKeys: nil,
		},
		{
			name: "With env",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					PodTemplate: mariadbv1alpha1.PodTemplate{
						InitContainers: []mariadbv1alpha1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep 1",
								},
								Env: []mariadbv1alpha1.EnvVar{
									{
										Name:  "TEST",
										Value: "TEST",
									},
									{
										Name:  "FOO",
										Value: "FOO",
									},
									{
										Name:  "BAR",
										Value: "BAR",
									},
								},
							},
						},
					},
				},
			},
			wantName:            "busybox",
			wantEnvKeys:         []string{"TEST", "FOO", "BAR"},
			wantVolumeMountKeys: nil,
		},
		{
			name: "With volumeMount",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Port: 3306,
					PodTemplate: mariadbv1alpha1.PodTemplate{
						InitContainers: []mariadbv1alpha1.Container{
							{
								Name:  "busybox",
								Image: "busybox",
								Command: []string{
									"sh",
									"-c",
									"sleep 1",
								},
								VolumeMounts: []mariadbv1alpha1.VolumeMount{
									{
										Name:      "TEST",
										MountPath: "/test",
									},
									{
										Name:      "FOO",
										MountPath: "/foo",
									},
									{
										Name:      "BAR",
										MountPath: "/bar",
									},
								},
							},
						},
					},
				},
			},
			wantName:            "busybox",
			wantEnvKeys:         nil,
			wantVolumeMountKeys: []string{"TEST", "FOO", "BAR"},
		},
	}

	builder := newDefaultTestBuilder(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initContainers, err := builder.mariadbInitContainers(tt.mariadb)
			if err != nil {
				t.Fatalf("unexpected error building init containers: %v", err)
			}

			initContainer := initContainers[0]

			if initContainer.Name != tt.wantName {
				t.Errorf("unexpected name:\nexpected:\n%s\ngot:\n%s\n", tt.wantName, initContainer.Name)
			}
			if tt.wantEnvKeys != nil {
				idx := datastructures.NewIndex(initContainer.Env, func(env corev1.EnvVar) string {
					return env.Name
				})
				if !datastructures.AllExists(idx, tt.wantEnvKeys...) {
					t.Errorf("expected env keys \"%v\" to exist", tt.wantEnvKeys)
				}
			}
			if tt.wantVolumeMountKeys != nil {
				idx := datastructures.NewIndex(initContainer.VolumeMounts, func(volumeMount corev1.VolumeMount) string {
					return volumeMount.Name
				})
				if !datastructures.AllExists(idx, tt.wantVolumeMountKeys...) {
					t.Errorf("expected volumeMount keys \"%s\" to exist", tt.wantVolumeMountKeys)
				}
			}
		})
	}
}

func TestMaxscaleContainers(t *testing.T) {
	tests := []struct {
		name        string
		maxscale    *mariadbv1alpha1.MaxScale
		wantCommand []string
		wantArgs    []string
	}{
		{
			name: "Without custom command and args",
			maxscale: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{},
				},
			},
			wantCommand: []string{"maxscale"},
			wantArgs:    []string{"--config", "/etc/config/maxscale.cnf", "-dU", "maxscale", "-l", "stdout"},
		},
		{
			name: "With custom command",
			maxscale: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Command: []string{"maxscale-test"},
					},
				},
			},
			wantCommand: []string{"maxscale-test"},
			wantArgs:    []string{"--config", "/etc/config/maxscale.cnf", "-dU", "maxscale", "-l", "stdout"},
		},
		{
			name: "With custom command and args",
			maxscale: &mariadbv1alpha1.MaxScale{
				Spec: mariadbv1alpha1.MaxScaleSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						Command: []string{"maxscale-test"},
						Args:    []string{"--test", "--unit"},
					},
				},
			},
			wantCommand: []string{"maxscale-test"},
			wantArgs:    []string{"--test", "--unit"},
		},
	}

	builder := newDefaultTestBuilder(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			containers, err := builder.maxscaleContainers(tt.maxscale)
			if err != nil {
				t.Fatalf("unexpected error building containers: %v", err)
			}

			container := containers[0]

			if !reflect.DeepEqual(container.Command[0], tt.wantCommand[0]) {
				t.Errorf("unexpected result:\nexpected:\n%s\ngot:\n%s\n", tt.wantCommand[0], container.Command[0])
			}

			if !reflect.DeepEqual(tt.wantArgs, container.Args) {
				t.Errorf("expected env keys \"%v\" to exist\n got:\"%v\"", tt.wantArgs, container.Args)
			}
		})
	}
}

func TestMariadbStorageVolumeMount(t *testing.T) {
	tests := []struct {
		name        string
		mariadb     *mariadbv1alpha1.MariaDB
		wantSubPath string
	}{
		{
			name:        "no galera configured",
			mariadb:     &mariadbv1alpha1.MariaDB{Spec: mariadbv1alpha1.MariaDBSpec{}},
			wantSubPath: "",
		},
		{
			name: "galera enabled reuse disabled",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Config: mariadbv1alpha1.GaleraConfig{
								ReuseStorageVolume: ptr.To(false),
							},
						},
					},
				},
			},
			wantSubPath: "",
		},
		{
			name: "galera enabled reuse enabled",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Config: mariadbv1alpha1.GaleraConfig{
								ReuseStorageVolume: ptr.To(true),
							},
						},
					},
				},
			},
			wantSubPath: StorageVolume,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := mariadbStorageVolumeMount(tt.mariadb)
			if vm.SubPath != tt.wantSubPath {
				t.Fatalf("unexpected SubPath: want=%q got=%q", tt.wantSubPath, vm.SubPath)
			}
		})
	}
}

func defaultEnv(overrides []corev1.EnvVar) []corev1.EnvVar {
	mysqlTcpPort := corev1.EnvVar{
		Name:  "MYSQL_TCP_PORT",
		Value: strconv.Itoa(0),
	}
	clusterName := corev1.EnvVar{
		Name:  "CLUSTER_NAME",
		Value: "cluster.local",
	}
	mariadbName := corev1.EnvVar{
		Name:  "MARIADB_NAME",
		Value: "",
	}
	mariadbRootPassword := corev1.EnvVar{
		Name: "MARIADB_ROOT_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{},
		},
	}
	mysqlInitdbSkipTzinfo := corev1.EnvVar{
		Name:  "MYSQL_INITDB_SKIP_TZINFO",
		Value: "1",
	}
	defaults := map[string]corev1.EnvVar{
		mysqlTcpPort.Name:          mysqlTcpPort,
		clusterName.Name:           clusterName,
		mariadbName.Name:           mariadbName,
		mariadbRootPassword.Name:   mariadbRootPassword,
		mysqlInitdbSkipTzinfo.Name: mysqlInitdbSkipTzinfo,
	}
	for _, override := range overrides {
		if _, ok := defaults[override.Name]; ok {
			defaults[override.Name] = override
		}
		if override.Name == "MARIADB_ALLOW_EMPTY_ROOT_PASSWORD" {
			defaults[mariadbRootPassword.Name] = override
		}
	}

	return []corev1.EnvVar{
		defaults[mysqlTcpPort.Name],
		{
			Name:  "MARIADB_ROOT_HOST",
			Value: "%",
		},
		defaults[clusterName.Name],
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		defaults[mariadbName.Name],
		defaults[mariadbRootPassword.Name],
		defaults[mysqlInitdbSkipTzinfo.Name],
	}
}

func removeEnv(env []corev1.EnvVar, key string) []corev1.EnvVar {
	var result []corev1.EnvVar
	for _, e := range env {
		if e.Name != key {
			result = append(result, e)
		}
	}
	return result
}

func sortEnvVars(env []corev1.EnvVar) []corev1.EnvVar {
	sortedEnv := make([]corev1.EnvVar, len(env))
	copy(sortedEnv, env)
	sort.SliceStable(sortedEnv, func(i, j int) bool {
		return sortedEnv[i].Name < sortedEnv[j].Name
	})
	return sortedEnv
}
