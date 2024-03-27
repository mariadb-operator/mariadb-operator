package builder

import (
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func TestLivenessProbe(t *testing.T) {
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
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB partial",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &corev1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       5,
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
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB full",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &corev1.Probe{
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
							PeriodSeconds:       5,
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
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB replication empty without probes",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							ProbesEnabled: ptr.To(false),
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
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB replication empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							ProbesEnabled: ptr.To(true),
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
							"/etc/probes/replication.sh",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB replication partial",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							ProbesEnabled: ptr.To(true),
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &corev1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       5,
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
							"/etc/probes/replication.sh",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB replication full",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							ProbesEnabled: ptr.To(true),
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								Exec: &corev1.ExecAction{
									Command: []string{
										"bash",
										"-c",
										"/etc/probes/replication-custom.sh",
									},
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       5,
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
							"/etc/probes/replication.sh",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB Galera empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.GaleraAgent{
								Port: 5555,
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
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB Galera partial",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.GaleraAgent{
								Port: 5555,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &corev1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       5,
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
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB Galera full",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.GaleraAgent{
								Port: 5555,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/liveness-custom",
									Port: intstr.FromInt(5555),
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       5,
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
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := mariadbLivenessProbe(tt.mariadb)
			if !reflect.DeepEqual(tt.wantProbe, probe) {
				t.Errorf("unexpected result:\nexpected:\n%s\ngot:\n%s\n", tt.wantProbe, probe)
			}
		})
	}
}

func TestReadinessProbe(t *testing.T) {
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
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB partial",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &corev1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       5,
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
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB full",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &corev1.Probe{
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
							PeriodSeconds:       5,
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
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB replication empty without probes",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							ProbesEnabled: ptr.To(false),
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
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB replication empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							ProbesEnabled: ptr.To(true),
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
							"/etc/probes/replication.sh",
						},
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB replication partial",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							ProbesEnabled: ptr.To(true),
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &corev1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       5,
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
							"/etc/probes/replication.sh",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB replication full",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replication: &mariadbv1alpha1.Replication{
						Enabled: true,
						ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
							ProbesEnabled: ptr.To(true),
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								Exec: &corev1.ExecAction{
									Command: []string{
										"bash",
										"-c",
										"/etc/probes/replication-custom.sh",
									},
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       5,
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
							"/etc/probes/replication.sh",
						},
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB Galera empty",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.GaleraAgent{
								Port: 5555,
							},
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB Galera partial",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.GaleraAgent{
								Port: 5555,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &corev1.Probe{
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       5,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       5,
			},
		},
		{
			name: "MariaDB Galera full",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					Galera: &mariadbv1alpha1.Galera{
						Enabled: true,
						GaleraSpec: mariadbv1alpha1.GaleraSpec{
							Agent: mariadbv1alpha1.GaleraAgent{
								Port: 5555,
							},
						},
					},
					ContainerTemplate: mariadbv1alpha1.ContainerTemplate{
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/readiness-custom",
									Port: intstr.FromInt(5555),
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       5,
						},
					},
				},
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readiness",
						Port: intstr.FromInt(5555),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := mariadbReadinessProbe(tt.mariadb)
			if !reflect.DeepEqual(tt.wantProbe, probe) {
				t.Errorf("unexpected result:\nexpected:\n%s\ngot:\n%s\n", tt.wantProbe, probe)
			}
		})
	}
}
