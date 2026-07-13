package builder

import (
	"fmt"
	"reflect"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	galeraresources "github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/galera/resources"
	kadapter "github.com/mariadb-operator/mariadb-operator/v26/pkg/kubernetes/adapter"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

var containerEnvGroupOrder = []mariadbv1alpha1.ContainerEnvGroup{
	mariadbv1alpha1.ContainerEnvGroupRuntime,
	mariadbv1alpha1.ContainerEnvGroupTLS,
	mariadbv1alpha1.ContainerEnvGroupReplication,
	mariadbv1alpha1.ContainerEnvGroupRootPassword,
	mariadbv1alpha1.ContainerEnvGroupUser,
}

var containerVolumeMountGroupOrder = []mariadbv1alpha1.ContainerVolumeMountGroup{
	mariadbv1alpha1.ContainerVolumeMountGroupConfig,
	mariadbv1alpha1.ContainerVolumeMountGroupTLS,
	mariadbv1alpha1.ContainerVolumeMountGroupPointInTimeRecovery,
	mariadbv1alpha1.ContainerVolumeMountGroupStorage,
	mariadbv1alpha1.ContainerVolumeMountGroupReplication,
	mariadbv1alpha1.ContainerVolumeMountGroupAgentAuth,
	mariadbv1alpha1.ContainerVolumeMountGroupServiceAccount,
	mariadbv1alpha1.ContainerVolumeMountGroupGalera,
	mariadbv1alpha1.ContainerVolumeMountGroupUser,
}

func validateContainerInheritance(container *mariadbv1alpha1.Container) (mariadbv1alpha1.ContainerInheritancePolicy, error) {
	if container.Inheritance == nil {
		return mariadbv1alpha1.ContainerInheritanceLegacy, nil
	}

	inheritance := container.Inheritance
	policy := inheritance.Policy
	if policy == "" {
		policy = mariadbv1alpha1.ContainerInheritanceLegacy
	}

	switch policy {
	case mariadbv1alpha1.ContainerInheritanceLegacy, mariadbv1alpha1.ContainerInheritanceIsolated:
		if len(inheritance.Env) > 0 || len(inheritance.VolumeMounts) > 0 {
			return "", fmt.Errorf("container inheritance policy %q cannot select env or volumeMount groups", policy)
		}
	case mariadbv1alpha1.ContainerInheritanceSelected:
		if len(inheritance.Env) == 0 && len(inheritance.VolumeMounts) == 0 {
			return "", fmt.Errorf("container inheritance policy %q must select at least one env or volumeMount group", policy)
		}
	default:
		return "", fmt.Errorf("unsupported container inheritance policy %q", policy)
	}

	return policy, nil
}

func selectedContainerEnv(mariadb *mariadbv1alpha1.MariaDB,
	groups []mariadbv1alpha1.ContainerEnvGroup) ([]corev1.EnvVar, error) {
	selected, err := selectedEnvGroups(groups)
	if err != nil {
		return nil, err
	}

	baseMariaDB := mariadb.DeepCopy()
	baseMariaDB.Spec.Env = nil
	if baseMariaDB.Spec.TLS == nil {
		baseMariaDB.Spec.TLS = &mariadbv1alpha1.TLS{Enabled: true}
	}
	baseEnv, err := mariadbEnv(baseMariaDB)
	if err != nil {
		return nil, err
	}

	var env []corev1.EnvVar
	for _, group := range containerEnvGroupOrder {
		if !selected[group] {
			continue
		}
		groupEnv, available := containerEnvGroup(mariadb, baseEnv, group)
		if !available {
			return nil, fmt.Errorf("container env inheritance group %q is not available for this MariaDB", group)
		}
		env = append(env, groupEnv...)
	}
	return env, nil
}

func selectedEnvGroups(groups []mariadbv1alpha1.ContainerEnvGroup) (map[mariadbv1alpha1.ContainerEnvGroup]bool, error) {
	selected := make(map[mariadbv1alpha1.ContainerEnvGroup]bool, len(groups))
	known := make(map[mariadbv1alpha1.ContainerEnvGroup]bool, len(containerEnvGroupOrder))
	for _, group := range containerEnvGroupOrder {
		known[group] = true
	}
	for _, group := range groups {
		if !known[group] {
			return nil, fmt.Errorf("unsupported container env inheritance group %q", group)
		}
		if selected[group] {
			return nil, fmt.Errorf("duplicate container env inheritance group %q", group)
		}
		selected[group] = true
	}
	return selected, nil
}

func containerEnvGroup(mariadb *mariadbv1alpha1.MariaDB, baseEnv []corev1.EnvVar,
	group mariadbv1alpha1.ContainerEnvGroup) ([]corev1.EnvVar, bool) {
	var names map[string]bool
	switch group {
	case mariadbv1alpha1.ContainerEnvGroupRuntime:
		names = stringSet(
			"MYSQL_TCP_PORT",
			"MARIADB_ROOT_HOST",
			"CLUSTER_NAME",
			"POD_NAME",
			"POD_NAMESPACE",
			"POD_IP",
			"MARIADB_NAME",
			"MYSQL_INITDB_SKIP_TZINFO",
		)
	case mariadbv1alpha1.ContainerEnvGroupTLS:
		if !containerTLSEnabled(mariadb) {
			return nil, false
		}
		names = stringSet(
			"TLS_ENABLED",
			"TLS_CA_CERT_PATH",
			"TLS_SERVER_CERT_PATH",
			"TLS_SERVER_KEY_PATH",
			"TLS_CLIENT_CERT_PATH",
			"TLS_CLIENT_KEY_PATH",
			"WSREP_SST_OPT_REMOTE_AUTH",
		)
	case mariadbv1alpha1.ContainerEnvGroupReplication:
		if !mariadb.IsReplicationEnabled() {
			return nil, false
		}
		names = stringSet(
			"MARIADB_REPL_ENABLED",
			"MARIADB_REPL_GTID_STRICT_MODE",
			"MARIADB_REPL_GTID_DOMAIN_ID",
			"MARIADB_REPL_SERVER_ID_START_INDEX",
			"MARIADB_REPL_SEMI_SYNC_ENABLED",
			"MARIADB_REPL_SEMI_SYNC_MASTER_TIMEOUT",
			"MARIADB_REPL_SEMI_SYNC_MASTER_WAIT_POINT",
			"MARIADB_REPL_SYNC_BINLOG",
		)
	case mariadbv1alpha1.ContainerEnvGroupRootPassword:
		names = stringSet("MARIADB_ROOT_PASSWORD", "MARIADB_ALLOW_EMPTY_ROOT_PASSWORD")
	case mariadbv1alpha1.ContainerEnvGroupUser:
		if len(mariadb.Spec.Env) == 0 {
			return nil, false
		}
		return kadapter.ToKubernetesSlice(mariadb.Spec.Env), true
	default:
		return nil, false
	}

	var env []corev1.EnvVar
	for _, envVar := range baseEnv {
		if names[envVar.Name] {
			env = append(env, envVar)
		}
	}
	return env, len(env) > 0
}

func selectedContainerVolumeMounts(mariadb *mariadbv1alpha1.MariaDB,
	groups []mariadbv1alpha1.ContainerVolumeMountGroup, opts ...mariadbPodOpt) ([]corev1.VolumeMount, error) {
	selected, err := selectedVolumeMountGroups(groups)
	if err != nil {
		return nil, err
	}
	mariadbOpts := newMariadbPodOpts(opts...)

	var volumeMounts []corev1.VolumeMount
	for _, group := range containerVolumeMountGroupOrder {
		if !selected[group] {
			continue
		}
		groupVolumeMounts, available, err := containerVolumeMountGroup(mariadb, mariadbOpts, group)
		if err != nil {
			return nil, err
		}
		if !available {
			return nil, fmt.Errorf("container volumeMount inheritance group %q is not available for this MariaDB", group)
		}
		volumeMounts = append(volumeMounts, groupVolumeMounts...)
	}
	return volumeMounts, nil
}

func selectedVolumeMountGroups(groups []mariadbv1alpha1.ContainerVolumeMountGroup) (
	map[mariadbv1alpha1.ContainerVolumeMountGroup]bool, error) {
	selected := make(map[mariadbv1alpha1.ContainerVolumeMountGroup]bool, len(groups))
	known := make(map[mariadbv1alpha1.ContainerVolumeMountGroup]bool, len(containerVolumeMountGroupOrder))
	for _, group := range containerVolumeMountGroupOrder {
		known[group] = true
	}
	for _, group := range groups {
		if !known[group] {
			return nil, fmt.Errorf("unsupported container volumeMount inheritance group %q", group)
		}
		if selected[group] {
			return nil, fmt.Errorf("duplicate container volumeMount inheritance group %q", group)
		}
		selected[group] = true
	}
	return selected, nil
}

func containerVolumeMountGroup(mariadb *mariadbv1alpha1.MariaDB, opts *mariadbPodOpts,
	group mariadbv1alpha1.ContainerVolumeMountGroup) ([]corev1.VolumeMount, bool, error) {
	switch group {
	case mariadbv1alpha1.ContainerVolumeMountGroupConfig:
		return []corev1.VolumeMount{{Name: ConfigVolume, MountPath: ConfigMountPath}}, true, nil
	case mariadbv1alpha1.ContainerVolumeMountGroupTLS:
		return containerTLSVolumeMountGroup(mariadb)
	case mariadbv1alpha1.ContainerVolumeMountGroupPointInTimeRecovery:
		return containerPointInTimeRecoveryVolumeMountGroup(opts)
	case mariadbv1alpha1.ContainerVolumeMountGroupStorage:
		return []corev1.VolumeMount{mariadbStorageVolumeMount(mariadb)}, true, nil
	case mariadbv1alpha1.ContainerVolumeMountGroupReplication:
		return containerReplicationVolumeMountGroup(mariadb)
	case mariadbv1alpha1.ContainerVolumeMountGroupAgentAuth:
		return containerAgentAuthVolumeMountGroup(mariadb, opts)
	case mariadbv1alpha1.ContainerVolumeMountGroupServiceAccount:
		return containerServiceAccountVolumeMountGroup(mariadb, opts)
	case mariadbv1alpha1.ContainerVolumeMountGroupGalera:
		return containerGaleraVolumeMountGroup(mariadb, opts)
	case mariadbv1alpha1.ContainerVolumeMountGroupUser:
		return containerUserVolumeMountGroup(mariadb)
	default:
		return nil, false, nil
	}
}

func containerTLSVolumeMountGroup(mariadb *mariadbv1alpha1.MariaDB) ([]corev1.VolumeMount, bool, error) {
	if !containerTLSEnabled(mariadb) {
		return nil, false, nil
	}
	tlsMariaDB := mariadb
	if mariadb.Spec.TLS == nil {
		tlsMariaDB = mariadb.DeepCopy()
		tlsMariaDB.Spec.TLS = &mariadbv1alpha1.TLS{Enabled: true}
	}
	_, volumeMounts := mariadbTLSVolumes(tlsMariaDB)
	return volumeMounts, len(volumeMounts) > 0, nil
}

func containerPointInTimeRecoveryVolumeMountGroup(opts *mariadbPodOpts) ([]corev1.VolumeMount, bool, error) {
	if opts.pointInTimeRecovery == nil {
		return nil, false, nil
	}
	_, s3VolumeMounts := s3Volumes(opts.pointInTimeRecovery.Spec.PointInTimeRecoveryStorage.S3)
	_, absVolumeMounts := absVolumes(opts.pointInTimeRecovery.Spec.PointInTimeRecoveryStorage.AzureBlob)
	volumeMounts := append(s3VolumeMounts, absVolumeMounts...)
	return volumeMounts, len(volumeMounts) > 0, nil
}

func containerReplicationVolumeMountGroup(mariadb *mariadbv1alpha1.MariaDB) ([]corev1.VolumeMount, bool, error) {
	if !mariadb.IsReplicationEnabled() {
		return nil, false, nil
	}
	return []corev1.VolumeMount{{Name: MariadbConfigVolume, MountPath: MariadbConfigMountPath}}, true, nil
}

func containerAgentAuthVolumeMountGroup(mariadb *mariadbv1alpha1.MariaDB,
	opts *mariadbPodOpts) ([]corev1.VolumeMount, bool, error) {
	if !mariadb.IsHAEnabled() || !opts.includeDataPlane {
		return nil, false, nil
	}
	_, agent, err := mariadb.GetDataPlaneAgent()
	if err != nil {
		return nil, false, fmt.Errorf("error getting data-plane agent: %v", err)
	}
	basicAuth := ptr.Deref(agent.BasicAuth, mariadbv1alpha1.BasicAuth{})
	if !basicAuth.Enabled || reflect.ValueOf(basicAuth.PasswordSecretKeyRef).IsZero() {
		return nil, false, nil
	}
	return []corev1.VolumeMount{{Name: AgentAuthVolume, MountPath: AgentAuthVolumeMount}}, true, nil
}

func containerServiceAccountVolumeMountGroup(mariadb *mariadbv1alpha1.MariaDB,
	opts *mariadbPodOpts) ([]corev1.VolumeMount, bool, error) {
	if !mariadb.IsHAEnabled() || !opts.includeDataPlane || !opts.includeServiceAccount {
		return nil, false, nil
	}
	_, volumeMount := serviceAccountVolumes()
	return []corev1.VolumeMount{volumeMount}, true, nil
}

func containerGaleraVolumeMountGroup(mariadb *mariadbv1alpha1.MariaDB,
	opts *mariadbPodOpts) ([]corev1.VolumeMount, bool, error) {
	if !mariadb.IsGaleraEnabled() || !opts.includeGaleraConfig {
		return nil, false, nil
	}
	volumeMount := corev1.VolumeMount{MountPath: galeraresources.GaleraConfigMountPath}
	galera := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{})
	if ptr.Deref(galera.Config.ReuseStorageVolume, false) {
		volumeMount.Name = StorageVolume
		volumeMount.SubPath = galeraresources.GaleraConfigVolume
	} else {
		volumeMount.Name = galeraresources.GaleraConfigVolume
	}
	return []corev1.VolumeMount{volumeMount}, true, nil
}

func containerUserVolumeMountGroup(mariadb *mariadbv1alpha1.MariaDB) ([]corev1.VolumeMount, bool, error) {
	if len(mariadb.Spec.VolumeMounts) == 0 {
		return nil, false, nil
	}
	return kadapter.ToKubernetesSlice(mariadb.Spec.VolumeMounts), true, nil
}

func containerTLSEnabled(mariadb *mariadbv1alpha1.MariaDB) bool {
	return mariadb.Spec.TLS == nil || mariadb.IsTLSEnabled()
}

func validateContainerEnvAndMounts(env []corev1.EnvVar, volumeMounts []corev1.VolumeMount) error {
	envNames := make(map[string]bool, len(env))
	for _, envVar := range env {
		if envNames[envVar.Name] {
			return fmt.Errorf("duplicate container environment variable %q", envVar.Name)
		}
		envNames[envVar.Name] = true
	}

	mountPaths := make(map[string]bool, len(volumeMounts))
	for _, volumeMount := range volumeMounts {
		if mountPaths[volumeMount.MountPath] {
			return fmt.Errorf("duplicate container volumeMount path %q", volumeMount.MountPath)
		}
		mountPaths[volumeMount.MountPath] = true
	}
	return nil
}

func stringSet(values ...string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
