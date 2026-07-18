package builder

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	kadapter "github.com/mariadb-operator/mariadb-operator/v26/pkg/kubernetes/adapter"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestExtraContainerLegacyInheritance(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	mariadb := testContainerInheritanceMariaDB()
	explicitEnv := mariadbv1alpha1.EnvVar{Name: "EXPLICIT", Value: "value"}
	explicitMount := mariadbv1alpha1.VolumeMount{Name: "explicit", MountPath: "/explicit"}

	legacyValues := []*mariadbv1alpha1.ContainerInheritance{
		nil,
		{},
		{Policy: mariadbv1alpha1.ContainerInheritanceLegacy},
	}
	for _, inheritance := range legacyValues {
		container, err := builder.buildContainer(mariadb, &mariadbv1alpha1.Container{
			Image:        "busybox:1.36",
			Env:          []mariadbv1alpha1.EnvVar{explicitEnv},
			VolumeMounts: []mariadbv1alpha1.VolumeMount{explicitMount},
			Inheritance:  inheritance,
		})
		if err != nil {
			t.Fatalf("unexpected error building legacy container: %v", err)
		}

		wantEnv, err := mariadbEnv(mariadb)
		if err != nil {
			t.Fatalf("unexpected error building expected env: %v", err)
		}
		wantEnv = append(wantEnv, explicitEnv.ToKubernetesType())
		if !reflect.DeepEqual(container.Env, wantEnv) {
			t.Errorf("legacy env changed:\nwant: %#v\ngot:  %#v", wantEnv, container.Env)
		}

		wantMounts, err := mariadbVolumeMounts(mariadb)
		if err != nil {
			t.Fatalf("unexpected error building expected volume mounts: %v", err)
		}
		wantMounts = append(wantMounts, explicitMount.ToKubernetesType())
		if !reflect.DeepEqual(container.VolumeMounts, wantMounts) {
			t.Errorf("legacy volume mounts changed:\nwant: %#v\ngot:  %#v", wantMounts, container.VolumeMounts)
		}
	}
}

func TestExtraContainerIsolationAndSecurityContext(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	mariadb := testContainerInheritanceMariaDB()
	isolated := mariadbv1alpha1.Container{
		Name:  "isolated",
		Image: "busybox:1.36",
		Env: []mariadbv1alpha1.EnvVar{
			{Name: "EXPLICIT", Value: "value"},
			{
				Name: "EXPLICIT_SECRET",
				ValueFrom: &mariadbv1alpha1.EnvVarSource{
					SecretKeyRef: ptr.To(testSecretKeySelector("explicit-secret", "value")),
				},
			},
		},
		VolumeMounts: []mariadbv1alpha1.VolumeMount{
			{Name: "explicit", MountPath: "/explicit", ReadOnly: true},
		},
		SecurityContext: &mariadbv1alpha1.SecurityContext{
			RunAsNonRoot:             ptr.To(true),
			AllowPrivilegeEscalation: ptr.To(false),
			Privileged:               ptr.To(false),
			ReadOnlyRootFilesystem:   ptr.To(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
			SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		},
		Resources: &mariadbv1alpha1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m")},
		},
		Inheritance: &mariadbv1alpha1.ContainerInheritance{
			Policy: mariadbv1alpha1.ContainerInheritanceIsolated,
		},
	}
	mariadb.Spec.InitContainers = []mariadbv1alpha1.Container{isolated}
	mariadb.Spec.SidecarContainers = []mariadbv1alpha1.Container{isolated}

	initContainers, err := builder.mariadbInitContainers(mariadb)
	if err != nil {
		t.Fatalf("unexpected error building init containers: %v", err)
	}
	containers, err := builder.mariadbContainers(mariadb)
	if err != nil {
		t.Fatalf("unexpected error building containers: %v", err)
	}

	for _, container := range []corev1.Container{initContainers[0], containers[len(containers)-1]} {
		if len(container.Env) != 2 || container.Env[0].Name != "EXPLICIT" || container.Env[1].Name != "EXPLICIT_SECRET" {
			t.Errorf("isolated container inherited unexpected env: %#v", container.Env)
		}
		if container.Env[1].ValueFrom == nil || container.Env[1].ValueFrom.SecretKeyRef == nil ||
			container.Env[1].ValueFrom.SecretKeyRef.Name != "explicit-secret" {
			t.Errorf("isolated container did not preserve its explicit Secret reference: %#v", container.Env[1])
		}
		if len(container.VolumeMounts) != 1 || container.VolumeMounts[0].MountPath != "/explicit" {
			t.Errorf("isolated container inherited unexpected volume mounts: %#v", container.VolumeMounts)
		}
		if container.SecurityContext == nil {
			t.Fatal("expected per-container security context")
		}
		if !ptr.Deref(container.SecurityContext.RunAsNonRoot, false) ||
			ptr.Deref(container.SecurityContext.AllowPrivilegeEscalation, true) ||
			ptr.Deref(container.SecurityContext.Privileged, true) ||
			!ptr.Deref(container.SecurityContext.ReadOnlyRootFilesystem, false) {
			t.Errorf("restricted security context was not preserved: %#v", container.SecurityContext)
		}
		if container.SecurityContext.SeccompProfile == nil ||
			container.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
			t.Errorf("RuntimeDefault seccomp profile was not preserved: %#v", container.SecurityContext.SeccompProfile)
		}
		if container.SecurityContext.Capabilities == nil ||
			!reflect.DeepEqual(container.SecurityContext.Capabilities.Drop, []corev1.Capability{"ALL"}) {
			t.Errorf("capability drop was not preserved: %#v", container.SecurityContext.Capabilities)
		}
		if container.Resources.Requests.Cpu().Cmp(resource.MustParse("10m")) != 0 {
			t.Errorf("explicit resources were not preserved: %#v", container.Resources)
		}
	}
}

func TestExtraContainerSelectedInheritanceIsDeterministic(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	mariadb := testContainerInheritanceMariaDB()
	mariadb.Spec.Replication = &mariadbv1alpha1.Replication{
		Enabled: true,
		ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
			ServerIDStartIndex: ptr.To(100),
		},
	}
	mariadb.Spec.Env = []mariadbv1alpha1.EnvVar{{Name: "USER_ENV", Value: "value"}}
	mariadb.Spec.VolumeMounts = []mariadbv1alpha1.VolumeMount{{Name: "user", MountPath: "/user"}}

	first := &mariadbv1alpha1.Container{
		Image: "busybox:1.36",
		Inheritance: &mariadbv1alpha1.ContainerInheritance{
			Policy: mariadbv1alpha1.ContainerInheritanceSelected,
			Env: []mariadbv1alpha1.ContainerEnvGroup{
				mariadbv1alpha1.ContainerEnvGroupUser,
				mariadbv1alpha1.ContainerEnvGroupRootPassword,
				mariadbv1alpha1.ContainerEnvGroupReplication,
				mariadbv1alpha1.ContainerEnvGroupTLS,
				mariadbv1alpha1.ContainerEnvGroupRuntime,
			},
			VolumeMounts: []mariadbv1alpha1.ContainerVolumeMountGroup{
				mariadbv1alpha1.ContainerVolumeMountGroupUser,
				mariadbv1alpha1.ContainerVolumeMountGroupReplication,
				mariadbv1alpha1.ContainerVolumeMountGroupStorage,
				mariadbv1alpha1.ContainerVolumeMountGroupTLS,
				mariadbv1alpha1.ContainerVolumeMountGroupConfig,
			},
		},
	}
	second := first.DeepCopy()
	secondInheritance := *first.Inheritance
	secondInheritance.Env = append([]mariadbv1alpha1.ContainerEnvGroup(nil), first.Inheritance.Env...)
	secondInheritance.VolumeMounts = append(
		[]mariadbv1alpha1.ContainerVolumeMountGroup(nil),
		first.Inheritance.VolumeMounts...,
	)
	second.Inheritance = &secondInheritance
	reverse(first.Inheritance.Env)
	reverse(first.Inheritance.VolumeMounts)

	firstBuilt, err := builder.buildContainer(mariadb, first)
	if err != nil {
		t.Fatalf("unexpected error building selected container: %v", err)
	}
	secondBuilt, err := builder.buildContainer(mariadb, second)
	if err != nil {
		t.Fatalf("unexpected error building selected container: %v", err)
	}
	if !reflect.DeepEqual(firstBuilt.Env, secondBuilt.Env) {
		t.Errorf("selected env order depends on input group order:\nfirst:  %#v\nsecond: %#v", firstBuilt.Env, secondBuilt.Env)
	}
	if !reflect.DeepEqual(firstBuilt.VolumeMounts, secondBuilt.VolumeMounts) {
		t.Errorf(
			"selected volume mount order depends on input group order:\nfirst:  %#v\nsecond: %#v",
			firstBuilt.VolumeMounts,
			secondBuilt.VolumeMounts,
		)
	}
	if !hasEnvVar(firstBuilt.Env, "MARIADB_ROOT_PASSWORD") || !hasEnvVar(firstBuilt.Env, "MARIADB_REPL_ENABLED") ||
		!hasEnvVar(firstBuilt.Env, "MARIADB_REPL_SERVER_ID_START_INDEX") ||
		!hasEnvVar(firstBuilt.Env, "TLS_ENABLED") || !hasEnvVar(firstBuilt.Env, "USER_ENV") {
		t.Errorf("selected env groups were not expanded: %#v", firstBuilt.Env)
	}
	if hasVolumeMount(firstBuilt.VolumeMounts, ServiceAccountMountPath) || hasVolumeMount(firstBuilt.VolumeMounts, AgentAuthVolumeMount) {
		t.Errorf("unselected privileged mounts were inherited: %#v", firstBuilt.VolumeMounts)
	}
}

func TestSelectedTLSInheritanceUsesTheAPIDefault(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	mariadb := testContainerInheritanceMariaDB()
	mariadb.Spec.TLS = nil
	container := selectedTestContainer(&mariadbv1alpha1.ContainerInheritance{
		Policy:       mariadbv1alpha1.ContainerInheritanceSelected,
		Env:          []mariadbv1alpha1.ContainerEnvGroup{mariadbv1alpha1.ContainerEnvGroupTLS},
		VolumeMounts: []mariadbv1alpha1.ContainerVolumeMountGroup{mariadbv1alpha1.ContainerVolumeMountGroupTLS},
	})

	built, err := builder.buildContainer(mariadb, container)
	if err != nil {
		t.Fatalf("unexpected error building defaulted TLS inheritance: %v", err)
	}
	if !hasEnvVar(built.Env, "TLS_ENABLED") {
		t.Errorf("defaulted TLS env group was not expanded: %#v", built.Env)
	}
	if len(built.VolumeMounts) == 0 {
		t.Error("defaulted TLS volumeMount group was not expanded")
	}
}

func TestSelectedGaleraAndAgentMountInheritance(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	mariadb := testContainerInheritanceMariaDB()
	mariadb.Spec.Galera = &mariadbv1alpha1.Galera{
		Enabled: true,
		GaleraSpec: mariadbv1alpha1.GaleraSpec{
			Agent: mariadbv1alpha1.Agent{
				BasicAuth: &mariadbv1alpha1.BasicAuth{
					Enabled: true,
					PasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: testSecretKeySelector("agent-auth", "password"),
					},
				},
			},
			Config: mariadbv1alpha1.GaleraConfig{ReuseStorageVolume: ptr.To(true)},
		},
	}
	container := selectedTestContainer(&mariadbv1alpha1.ContainerInheritance{
		Policy: mariadbv1alpha1.ContainerInheritanceSelected,
		VolumeMounts: []mariadbv1alpha1.ContainerVolumeMountGroup{
			mariadbv1alpha1.ContainerVolumeMountGroupGalera,
			mariadbv1alpha1.ContainerVolumeMountGroupAgentAuth,
			mariadbv1alpha1.ContainerVolumeMountGroupServiceAccount,
		},
	})

	built, err := builder.buildContainer(mariadb, container)
	if err != nil {
		t.Fatalf("unexpected error building selected Galera inheritance: %v", err)
	}
	for _, mountPath := range []string{MariadbConfigMountPath, AgentAuthVolumeMount, ServiceAccountMountPath} {
		if !hasVolumeMount(built.VolumeMounts, mountPath) {
			t.Errorf("expected selected mount path %q in %#v", mountPath, built.VolumeMounts)
		}
	}
}

func TestSelectedPointInTimeRecoveryMountInheritance(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	mariadb := testContainerInheritanceMariaDB()
	pitr := &mariadbv1alpha1.PointInTimeRecovery{
		Spec: mariadbv1alpha1.PointInTimeRecoverySpec{
			PointInTimeRecoveryStorage: mariadbv1alpha1.PointInTimeRecoveryStorage{
				S3: &mariadbv1alpha1.S3{
					TLS: &mariadbv1alpha1.TLSConfig{
						Enabled:        true,
						CASecretKeyRef: ptr.To(testSecretKeySelector("s3-ca", "ca.crt")),
					},
				},
			},
		},
	}
	container := selectedTestContainer(&mariadbv1alpha1.ContainerInheritance{
		Policy: mariadbv1alpha1.ContainerInheritanceSelected,
		VolumeMounts: []mariadbv1alpha1.ContainerVolumeMountGroup{
			mariadbv1alpha1.ContainerVolumeMountGroupPointInTimeRecovery,
		},
	})

	built, err := builder.buildContainer(mariadb, container, withPointInTimeRecovery(pitr))
	if err != nil {
		t.Fatalf("unexpected error building selected point-in-time recovery inheritance: %v", err)
	}
	if !hasVolumeMount(built.VolumeMounts, S3PKIMountPath) {
		t.Errorf("point-in-time recovery TLS CA mount was not inherited: %#v", built.VolumeMounts)
	}
}

func TestExtraContainerInheritanceValidation(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	tests := []struct {
		name      string
		mariadb   *mariadbv1alpha1.MariaDB
		container *mariadbv1alpha1.Container
		wantError string
	}{
		{
			name:    "unknown policy",
			mariadb: testContainerInheritanceMariaDB(),
			container: selectedTestContainer(&mariadbv1alpha1.ContainerInheritance{
				Policy: "Unknown",
			}),
			wantError: "unsupported container inheritance policy",
		},
		{
			name:    "legacy with groups",
			mariadb: testContainerInheritanceMariaDB(),
			container: selectedTestContainer(&mariadbv1alpha1.ContainerInheritance{
				Policy: mariadbv1alpha1.ContainerInheritanceLegacy,
				Env:    []mariadbv1alpha1.ContainerEnvGroup{mariadbv1alpha1.ContainerEnvGroupRuntime},
			}),
			wantError: "cannot select env or volumeMount groups",
		},
		{
			name:    "selected without groups",
			mariadb: testContainerInheritanceMariaDB(),
			container: selectedTestContainer(&mariadbv1alpha1.ContainerInheritance{
				Policy: mariadbv1alpha1.ContainerInheritanceSelected,
			}),
			wantError: "must select at least one",
		},
		{
			name:    "duplicate group",
			mariadb: testContainerInheritanceMariaDB(),
			container: selectedTestContainer(&mariadbv1alpha1.ContainerInheritance{
				Policy: mariadbv1alpha1.ContainerInheritanceSelected,
				Env: []mariadbv1alpha1.ContainerEnvGroup{
					mariadbv1alpha1.ContainerEnvGroupRuntime,
					mariadbv1alpha1.ContainerEnvGroupRuntime,
				},
			}),
			wantError: "duplicate container env inheritance group",
		},
		{
			name:    "unavailable group",
			mariadb: &mariadbv1alpha1.MariaDB{Spec: mariadbv1alpha1.MariaDBSpec{TLS: &mariadbv1alpha1.TLS{Enabled: false}}},
			container: selectedTestContainer(&mariadbv1alpha1.ContainerInheritance{
				Policy: mariadbv1alpha1.ContainerInheritanceSelected,
				Env:    []mariadbv1alpha1.ContainerEnvGroup{mariadbv1alpha1.ContainerEnvGroupTLS},
			}),
			wantError: "is not available",
		},
		{
			name: "agent auth group with Kubernetes auth only",
			mariadb: testContainerInheritanceMariaDBWithAgent(mariadbv1alpha1.Agent{
				KubernetesAuth: &mariadbv1alpha1.KubernetesAuth{Enabled: true},
			}),
			container: selectedTestContainer(&mariadbv1alpha1.ContainerInheritance{
				Policy:       mariadbv1alpha1.ContainerInheritanceSelected,
				VolumeMounts: []mariadbv1alpha1.ContainerVolumeMountGroup{mariadbv1alpha1.ContainerVolumeMountGroupAgentAuth},
			}),
			wantError: "is not available",
		},
		{
			name: "agent auth group without a password Secret reference",
			mariadb: testContainerInheritanceMariaDBWithAgent(mariadbv1alpha1.Agent{
				BasicAuth: &mariadbv1alpha1.BasicAuth{Enabled: true},
			}),
			container: selectedTestContainer(&mariadbv1alpha1.ContainerInheritance{
				Policy:       mariadbv1alpha1.ContainerInheritanceSelected,
				VolumeMounts: []mariadbv1alpha1.ContainerVolumeMountGroup{mariadbv1alpha1.ContainerVolumeMountGroupAgentAuth},
			}),
			wantError: "is not available",
		},
		{
			name:    "duplicate authored env",
			mariadb: testContainerInheritanceMariaDB(),
			container: &mariadbv1alpha1.Container{
				Image: "busybox:1.36",
				Inheritance: &mariadbv1alpha1.ContainerInheritance{
					Policy: mariadbv1alpha1.ContainerInheritanceIsolated,
				},
				Env: []mariadbv1alpha1.EnvVar{{Name: "DUPLICATE"}, {Name: "DUPLICATE"}},
			},
			wantError: "duplicate container environment variable",
		},
		{
			name:    "selected env collision",
			mariadb: testContainerInheritanceMariaDB(),
			container: &mariadbv1alpha1.Container{
				Image: "busybox:1.36",
				Inheritance: &mariadbv1alpha1.ContainerInheritance{
					Policy: mariadbv1alpha1.ContainerInheritanceSelected,
					Env:    []mariadbv1alpha1.ContainerEnvGroup{mariadbv1alpha1.ContainerEnvGroupRuntime},
				},
				Env: []mariadbv1alpha1.EnvVar{{Name: "MYSQL_TCP_PORT"}},
			},
			wantError: "duplicate container environment variable",
		},
		{
			name:    "selected mount collision",
			mariadb: testContainerInheritanceMariaDB(),
			container: &mariadbv1alpha1.Container{
				Image: "busybox:1.36",
				Inheritance: &mariadbv1alpha1.ContainerInheritance{
					Policy:       mariadbv1alpha1.ContainerInheritanceSelected,
					VolumeMounts: []mariadbv1alpha1.ContainerVolumeMountGroup{mariadbv1alpha1.ContainerVolumeMountGroupStorage},
				},
				VolumeMounts: []mariadbv1alpha1.VolumeMount{{Name: "collision", MountPath: MariadbStorageMountPath}},
			},
			wantError: "duplicate container volumeMount path",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := builder.buildContainer(test.mariadb, test.container)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("expected error containing %q, got %v", test.wantError, err)
			}
		})
	}
}

func TestLegacyExtraContainersPreserveHistoricalOutputWithPodOptions(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	mariadb := testContainerInheritanceMariaDB()
	mariadb.Spec.Replication = &mariadbv1alpha1.Replication{
		Enabled: true,
		ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
			Agent: mariadbv1alpha1.Agent{
				BasicAuth: &mariadbv1alpha1.BasicAuth{
					Enabled: true,
					PasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: testSecretKeySelector("agent-auth", "password"),
					},
				},
			},
		},
	}
	container := mariadbv1alpha1.Container{
		Name:    "legacy-extra",
		Image:   "busybox:1.36",
		Command: []string{"sh", "-c"},
		Args:    []string{"true"},
		Env:     []mariadbv1alpha1.EnvVar{{Name: "EXPLICIT", Value: "value"}},
		VolumeMounts: []mariadbv1alpha1.VolumeMount{
			{Name: "explicit", MountPath: "/explicit", ReadOnly: true},
		},
	}
	pitr := &mariadbv1alpha1.PointInTimeRecovery{
		Spec: mariadbv1alpha1.PointInTimeRecoverySpec{
			PointInTimeRecoveryStorage: mariadbv1alpha1.PointInTimeRecoveryStorage{
				S3: &mariadbv1alpha1.S3{
					TLS: &mariadbv1alpha1.TLSConfig{
						Enabled:        true,
						CASecretKeyRef: ptr.To(testSecretKeySelector("pitr-ca", "ca.crt")),
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		opts []mariadbPodOpt
	}{
		{name: "default options"},
		{
			name: "PITR and restricted Pod options",
			opts: []mariadbPodOpt{
				withPointInTimeRecovery(pitr),
				withDataPlane(false),
				withServiceAccount(false),
				withExtraVolumeMounts([]corev1.VolumeMount{{Name: "optioned", MountPath: "/optioned"}}),
			},
		},
	}
	inheritanceValues := []*mariadbv1alpha1.ContainerInheritance{
		nil,
		{},
		{Policy: mariadbv1alpha1.ContainerInheritanceLegacy},
	}

	for _, inheritance := range inheritanceValues {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				legacyContainer := container
				legacyContainer.Inheritance = inheritance
				mariadb.Spec.InitContainers = []mariadbv1alpha1.Container{legacyContainer}
				mariadb.Spec.SidecarContainers = []mariadbv1alpha1.Container{legacyContainer}

				expected := historicalExtraContainer(t, mariadb, &legacyContainer)
				expectedBytes, err := json.Marshal(expected)
				if err != nil {
					t.Fatalf("unexpected error marshaling historical container: %v", err)
				}

				containers, err := builder.mariadbContainers(mariadb, test.opts...)
				if err != nil {
					t.Fatalf("unexpected error building legacy sidecar: %v", err)
				}
				initContainers, err := builder.mariadbInitContainers(mariadb, test.opts...)
				if err != nil {
					t.Fatalf("unexpected error building legacy init container: %v", err)
				}

				for _, actual := range []corev1.Container{containers[len(containers)-1], initContainers[0]} {
					actualBytes, err := json.Marshal(actual)
					if err != nil {
						t.Fatalf("unexpected error marshaling built container: %v", err)
					}
					if string(actualBytes) != string(expectedBytes) {
						t.Errorf("legacy container output changed:\nwant: %s\ngot:  %s", expectedBytes, actualBytes)
					}
					if hasVolumeMount(actual.VolumeMounts, S3PKIMountPath) || hasVolumeMount(actual.VolumeMounts, "/optioned") {
						t.Errorf("legacy container inherited Pod-option mounts: %#v", actual.VolumeMounts)
					}
				}
			})
		}
	}
}

func TestIsolatedContainersExcludeExpandedPodSecretsAndMounts(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	mariadb := testContainerInheritanceMariaDB()
	mariadb.Spec.Replication = &mariadbv1alpha1.Replication{
		Enabled: true,
		ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
			InitContainer: mariadbv1alpha1.InitContainer{Image: "operator:init"},
			Agent: mariadbv1alpha1.Agent{
				Image: "operator:agent",
				BasicAuth: &mariadbv1alpha1.BasicAuth{
					Enabled: true,
					PasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: testSecretKeySelector("agent-auth", "password"),
					},
				},
			},
		},
	}
	mariadb.Spec.Volumes = []mariadbv1alpha1.MariaDBVolume{
		{
			Name: "custom-secret",
			MariaDBVolumeSource: mariadbv1alpha1.MariaDBVolumeSource{
				VolumeSource: mariadbv1alpha1.VolumeSource{
					Secret: &mariadbv1alpha1.SecretVolumeSource{SecretName: "custom-secret"},
				},
			},
		},
	}
	mariadb.Spec.VolumeMounts = []mariadbv1alpha1.VolumeMount{{Name: "custom-secret", MountPath: "/custom-secret"}}
	isolated := mariadbv1alpha1.Container{
		Name:        "isolated",
		Image:       "busybox:1.36",
		Inheritance: &mariadbv1alpha1.ContainerInheritance{Policy: mariadbv1alpha1.ContainerInheritanceIsolated},
	}
	mariadb.Spec.InitContainers = []mariadbv1alpha1.Container{isolated}
	mariadb.Spec.SidecarContainers = []mariadbv1alpha1.Container{isolated}

	podTemplate, err := builder.mariadbPodTemplate(mariadb)
	if err != nil {
		t.Fatalf("unexpected error building expanded Pod template: %v", err)
	}
	customInit := podTemplate.Spec.InitContainers[0]
	customSidecar := podTemplate.Spec.Containers[len(podTemplate.Spec.Containers)-1]
	for _, container := range []corev1.Container{customInit, customSidecar} {
		if len(container.Env) != 0 {
			t.Errorf("isolated container inherited expanded Pod env: %#v", container.Env)
		}
		if len(container.VolumeMounts) != 0 {
			t.Errorf("isolated container inherited expanded Pod mounts: %#v", container.VolumeMounts)
		}
	}
}

func historicalExtraContainer(t *testing.T, mariadb *mariadbv1alpha1.MariaDB,
	container *mariadbv1alpha1.Container) corev1.Container {
	t.Helper()
	env, err := mariadbEnv(mariadb)
	if err != nil {
		t.Fatalf("unexpected error building historical env: %v", err)
	}
	env = append(env, kadapter.ToKubernetesSlice(container.Env)...)

	volumeMounts, err := mariadbVolumeMounts(mariadb)
	if err != nil {
		t.Fatalf("unexpected error building historical volume mounts: %v", err)
	}
	volumeMounts = append(volumeMounts, kadapter.ToKubernetesSlice(container.VolumeMounts)...)

	historical := corev1.Container{
		Name:         container.Name,
		Image:        container.Image,
		Command:      container.Command,
		Args:         container.Args,
		Env:          env,
		VolumeMounts: volumeMounts,
	}
	if container.Resources != nil {
		historical.Resources = container.Resources.ToKubernetesType()
	}
	return historical
}

func testContainerInheritanceMariaDB() *mariadbv1alpha1.MariaDB {
	return &mariadbv1alpha1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{Name: "mariadb", Namespace: "default"},
		Spec: mariadbv1alpha1.MariaDBSpec{
			Image: "mariadb:11.8",
			Port:  3306,
			RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
				SecretKeySelector: testSecretKeySelector("mariadb-root", "password"),
			},
			TLS: &mariadbv1alpha1.TLS{Enabled: true},
		},
	}
}

func testContainerInheritanceMariaDBWithAgent(agent mariadbv1alpha1.Agent) *mariadbv1alpha1.MariaDB {
	mariadb := testContainerInheritanceMariaDB()
	mariadb.Spec.Replication = &mariadbv1alpha1.Replication{
		Enabled: true,
		ReplicationSpec: mariadbv1alpha1.ReplicationSpec{
			Agent: agent,
		},
	}
	return mariadb
}

func testSecretKeySelector(name, key string) mariadbv1alpha1.SecretKeySelector {
	return mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{Name: name},
		Key:                  key,
	}
}

func selectedTestContainer(inheritance *mariadbv1alpha1.ContainerInheritance) *mariadbv1alpha1.Container {
	return &mariadbv1alpha1.Container{Image: "busybox:1.36", Inheritance: inheritance}
}

func hasEnvVar(env []corev1.EnvVar, name string) bool {
	for _, envVar := range env {
		if envVar.Name == name {
			return true
		}
	}
	return false
}

func hasVolumeMount(volumeMounts []corev1.VolumeMount, mountPath string) bool {
	for _, volumeMount := range volumeMounts {
		if volumeMount.MountPath == mountPath {
			return true
		}
	}
	return false
}

func reverse[T any](values []T) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
