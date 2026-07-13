package v1alpha1

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validateContainers(mariadb *mariadbv1alpha1.MariaDB) error {
	for index := range mariadb.Spec.InitContainers {
		container := &mariadb.Spec.InitContainers[index]
		path := field.NewPath("spec").Child("initContainers").Index(index)
		if err := validateContainer(mariadb, container, path); err != nil {
			return err
		}
	}
	for index := range mariadb.Spec.SidecarContainers {
		container := &mariadb.Spec.SidecarContainers[index]
		path := field.NewPath("spec").Child("sidecarContainers").Index(index)
		if err := validateContainer(mariadb, container, path); err != nil {
			return err
		}
	}
	return nil
}

func validateContainer(mariadb *mariadbv1alpha1.MariaDB, container *mariadbv1alpha1.Container, path *field.Path) error {
	if container.Inheritance == nil {
		return nil
	}

	inheritance := container.Inheritance
	policy := inheritance.Policy
	if policy == "" {
		policy = mariadbv1alpha1.ContainerInheritanceLegacy
	}

	switch policy {
	case mariadbv1alpha1.ContainerInheritanceLegacy, mariadbv1alpha1.ContainerInheritanceIsolated:
		if len(inheritance.Env) > 0 || len(inheritance.VolumeMounts) > 0 {
			return field.Invalid(
				path.Child("inheritance"),
				inheritance,
				fmt.Sprintf("policy %q cannot select env or volumeMount groups", policy),
			)
		}
	case mariadbv1alpha1.ContainerInheritanceSelected:
		if len(inheritance.Env) == 0 && len(inheritance.VolumeMounts) == 0 {
			return field.Invalid(
				path.Child("inheritance"),
				inheritance,
				"policy \"Selected\" must select at least one env or volumeMount group",
			)
		}
	default:
		return field.NotSupported(
			path.Child("inheritance").Child("policy"),
			inheritance.Policy,
			[]string{
				string(mariadbv1alpha1.ContainerInheritanceLegacy),
				string(mariadbv1alpha1.ContainerInheritanceIsolated),
				string(mariadbv1alpha1.ContainerInheritanceSelected),
			},
		)
	}

	if policy == mariadbv1alpha1.ContainerInheritanceLegacy {
		return nil
	}
	if err := validateContainerEnvGroups(mariadb, inheritance.Env, path.Child("inheritance").Child("env")); err != nil {
		return err
	}
	if err := validateContainerVolumeMountGroups(
		mariadb,
		inheritance.VolumeMounts,
		path.Child("inheritance").Child("volumeMounts"),
	); err != nil {
		return err
	}
	if err := validateContainerAuthoredValues(container, path); err != nil {
		return err
	}
	return nil
}

func validateContainerEnvGroups(mariadb *mariadbv1alpha1.MariaDB,
	groups []mariadbv1alpha1.ContainerEnvGroup, path *field.Path) error {
	seen := make(map[mariadbv1alpha1.ContainerEnvGroup]bool, len(groups))
	for index, group := range groups {
		if seen[group] {
			return field.Duplicate(path.Index(index), group)
		}
		seen[group] = true
		switch group {
		case mariadbv1alpha1.ContainerEnvGroupRuntime, mariadbv1alpha1.ContainerEnvGroupRootPassword:
		case mariadbv1alpha1.ContainerEnvGroupTLS:
			if !containerTLSEnabled(mariadb) {
				return unavailableContainerGroup(path.Index(index), group)
			}
		case mariadbv1alpha1.ContainerEnvGroupReplication:
			if !mariadb.IsReplicationEnabled() {
				return unavailableContainerGroup(path.Index(index), group)
			}
		case mariadbv1alpha1.ContainerEnvGroupUser:
			if len(mariadb.Spec.Env) == 0 {
				return unavailableContainerGroup(path.Index(index), group)
			}
		default:
			return field.NotSupported(path.Index(index), group, containerEnvGroupNames())
		}
	}
	return nil
}

func validateContainerVolumeMountGroups(mariadb *mariadbv1alpha1.MariaDB,
	groups []mariadbv1alpha1.ContainerVolumeMountGroup, path *field.Path) error {
	seen := make(map[mariadbv1alpha1.ContainerVolumeMountGroup]bool, len(groups))
	for index, group := range groups {
		if seen[group] {
			return field.Duplicate(path.Index(index), group)
		}
		seen[group] = true
		switch group {
		case mariadbv1alpha1.ContainerVolumeMountGroupConfig, mariadbv1alpha1.ContainerVolumeMountGroupStorage:
		case mariadbv1alpha1.ContainerVolumeMountGroupTLS:
			if !containerTLSEnabled(mariadb) {
				return unavailableContainerGroup(path.Index(index), group)
			}
		case mariadbv1alpha1.ContainerVolumeMountGroupReplication:
			if !mariadb.IsReplicationEnabled() {
				return unavailableContainerGroup(path.Index(index), group)
			}
		case mariadbv1alpha1.ContainerVolumeMountGroupAgentAuth,
			mariadbv1alpha1.ContainerVolumeMountGroupServiceAccount:
			if !mariadb.IsHAEnabled() {
				return unavailableContainerGroup(path.Index(index), group)
			}
		case mariadbv1alpha1.ContainerVolumeMountGroupGalera:
			if !mariadb.IsGaleraEnabled() {
				return unavailableContainerGroup(path.Index(index), group)
			}
		case mariadbv1alpha1.ContainerVolumeMountGroupPointInTimeRecovery:
			if !mariadb.IsPointInTimeRecoveryEnabled() {
				return unavailableContainerGroup(path.Index(index), group)
			}
		case mariadbv1alpha1.ContainerVolumeMountGroupUser:
			if len(mariadb.Spec.VolumeMounts) == 0 {
				return unavailableContainerGroup(path.Index(index), group)
			}
		default:
			return field.NotSupported(path.Index(index), group, containerVolumeMountGroupNames())
		}
	}
	return nil
}

func validateContainerAuthoredValues(container *mariadbv1alpha1.Container, path *field.Path) error {
	envNames := make(map[string]bool, len(container.Env))
	for index, envVar := range container.Env {
		if envNames[envVar.Name] {
			return field.Duplicate(path.Child("env").Index(index).Child("name"), envVar.Name)
		}
		envNames[envVar.Name] = true
	}

	mountPaths := make(map[string]bool, len(container.VolumeMounts))
	for index, volumeMount := range container.VolumeMounts {
		if mountPaths[volumeMount.MountPath] {
			return field.Duplicate(path.Child("volumeMounts").Index(index).Child("mountPath"), volumeMount.MountPath)
		}
		mountPaths[volumeMount.MountPath] = true
	}
	return nil
}

func unavailableContainerGroup(path *field.Path, group any) error {
	return field.Invalid(path, group, "the selected inheritance group is not available for this MariaDB")
}

func containerEnvGroupNames() []string {
	return []string{"Runtime", "TLS", "Replication", "RootPassword", "User"}
}

func containerVolumeMountGroupNames() []string {
	return []string{
		"Config",
		"TLS",
		"Storage",
		"Replication",
		"AgentAuth",
		"ServiceAccount",
		"Galera",
		"PointInTimeRecovery",
		"User",
	}
}

func containerTLSEnabled(mariadb *mariadbv1alpha1.MariaDB) bool {
	return mariadb.Spec.TLS == nil || mariadb.IsTLSEnabled()
}
