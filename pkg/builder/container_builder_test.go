package builder

import (
	"fmt"
	"reflect"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/command"
	"github.com/mariadb-operator/mariadb-operator/pkg/discovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func TestMariadbLivenessProbe(t *testing.T) {
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

func TestMaxScaleProbe(t *testing.T) {
	tests := []struct {
		name      string
		maxScale  *mariadbv1alpha1.MaxScale
		probe     *corev1.Probe
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
			probe: &corev1.Probe{},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/",
						Port: intstr.FromInt(8989),
					},
				},
				InitialDelaySeconds: 20,
				TimeoutSeconds:      5,
				PeriodSeconds:       5,
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
			probe: &corev1.Probe{
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       5,
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/",
						Port: intstr.FromInt(8989),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       5,
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
			probe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/custom",
						Port: intstr.FromInt(8989),
					},
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      10,
				PeriodSeconds:       5,
			},
			wantProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/custom",
						Port: intstr.FromInt(8989),
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
			probe := maxscaleProbe(tt.maxScale, tt.probe)
			if !reflect.DeepEqual(tt.wantProbe, probe) {
				t.Errorf("unexpected result:\nexpected:\n%s\ngot:\n%s\n", tt.wantProbe, probe)
			}
		})
	}
}

func TestMaxScaleCommand(t *testing.T) {
	mxs := mariadbv1alpha1.MaxScale{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-maxscale-command",
		},
	}
	builder := newDefaultTestBuilder(t)

	expectedCmd := command.NewCommand(
		[]string{
			"maxscale",
		},
		[]string{
			"--config",
			fmt.Sprintf("%s/%s", MaxscaleConfigMountPath, mxs.ConfigSecretKeyRef().Key),
			"-dU",
			"maxscale",
			"-l",
			"stdout",
		},
	)
	cmd, err := builder.maxscaleCommand(&mxs)
	if err != nil {
		t.Fatalf("unexpected error getting MaxScale command: %v", err)
	}
	if !reflect.DeepEqual(cmd, expectedCmd) {
		t.Error("unexpected MaxScale command")
	}

	resource := &metav1.APIResourceList{
		GroupVersion: "security.openshift.io/v1",
		APIResources: []metav1.APIResource{
			{
				Name: "securitycontextconstraints",
			},
		},
	}
	d, err := discovery.NewFakeDiscovery(true, resource)
	if err != nil {
		t.Fatalf("unexpected error getting discovery: %v", err)
	}
	builder = newTestBuilder(d)

	expectedCmd = command.NewBashCommand(
		[]string{
			fmt.Sprintf(
				"maxscale --config %s -dU $(id -u) -l stdout",
				fmt.Sprintf("%s/%s", MaxscaleConfigMountPath, mxs.ConfigSecretKeyRef().Key),
			),
		},
	)
	cmd, err = builder.maxscaleCommand(&mxs)
	if err != nil {
		t.Fatalf("unexpected error getting MaxScale enterprise command: %v", err)
	}
	if !reflect.DeepEqual(cmd, expectedCmd) {
		t.Error("unexpected MaxScale enterprise command")
	}
}

func TestContainerSecurityContext(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	tpl := &mariadbv1alpha1.ContainerTemplate{}

	container, err := builder.buildContainer("mariadb:10.6", corev1.PullIfNotPresent, tpl)
	if err != nil {
		t.Fatalf("unexpected error building container: %v", err)
	}
	if container.SecurityContext != nil {
		t.Error("expected SecurityContext to be nil")
	}

	tpl = &mariadbv1alpha1.ContainerTemplate{
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: ptr.To(mysqlUser),
		},
	}
	container, err = builder.buildContainer("mariadb:10.6", corev1.PullIfNotPresent, tpl)
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
	discovery, err := discovery.NewFakeDiscovery(false, resource)
	if err != nil {
		t.Fatalf("unexpected error getting discovery: %v", err)
	}
	builder = newTestBuilder(discovery)

	container, err = builder.buildContainer("mariadb:10.6", corev1.PullIfNotPresent, tpl)
	if err != nil {
		t.Fatalf("unexpected error building container: %v", err)
	}
	if container.SecurityContext != nil {
		t.Error("expected SecurityContext to be nil")
	}
}
