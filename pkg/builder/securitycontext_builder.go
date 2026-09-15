package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func (b *Builder) buildContainerSecurityContext(securityContext *mariadbv1alpha1.SecurityContext) (*corev1.SecurityContext, error) {
	sccExists, err := b.discovery.SecurityContextConstraintsExist()
	if err != nil {
		return nil, fmt.Errorf("error discovering SecurityContextConstraints: %v", err)
	}
	// Delegate SecurityContext assignment to OpenShift.
	// A SecurityContext is created based on SecurityContextConstraints.
	// See: https://redhat-connect.gitbook.io/certified-operator-guide/troubleshooting-and-resources/sccs
	if sccExists {
		return nil, nil
	}
	if securityContext != nil {
		return ptr.To(securityContext.ToKubernetesType()), nil
	}
	return nil, nil
}

func (b *Builder) buildPodSecurityContext(podSecurityContext *mariadbv1alpha1.PodSecurityContext) (*corev1.PodSecurityContext, error) {
	sccExists, err := b.discovery.SecurityContextConstraintsExist()
	if err != nil {
		return nil, fmt.Errorf("error discovering SecurityContextConstraints: %v", err)
	}
	// Delegate SecurityContext assignment to OpenShift.
	// A SecurityContext is created based on SecurityContextConstraints.
	// See: https://redhat-connect.gitbook.io/certified-operator-guide/troubleshooting-and-resources/sccs
	if sccExists {
		return nil, nil
	}
	if podSecurityContext != nil {
		return ptr.To(podSecurityContext.ToKubernetesType()), nil
	}
	return nil, nil
}

func (b *Builder) buildPodSecurityContextWithUserGroup(podSecurityContext *mariadbv1alpha1.PodSecurityContext,
	user, group int64) (*corev1.PodSecurityContext, error) {
	sccExists, err := b.discovery.SecurityContextConstraintsExist()
	if err != nil {
		return nil, fmt.Errorf("error discovering SecurityContextConstraints: %v", err)
	}
	// Delegate SecurityContext assignment to OpenShift.
	// A SecurityContext is created based on SecurityContextConstraints.
	// See: https://redhat-connect.gitbook.io/certified-operator-guide/troubleshooting-and-resources/sccs
	if sccExists {
		return nil, nil
	}
	if podSecurityContext != nil {
		return ptr.To(podSecurityContext.ToKubernetesType()), nil
	}

	return &corev1.PodSecurityContext{
		RunAsNonRoot: ptr.To(true),
		RunAsUser:    &user,
		RunAsGroup:   &group,
		FSGroup:      &group,
	}, nil
}

// inheritMariadbSELinuxOptions makes a Job that co-mounts the MariaDB data volume
// (e.g. a PhysicalBackup Job) share the SELinux context of the MariaDB Pod.
//
// An SELinux context explicitly configured on the Job's own PodSecurityContext
// takes precedence and is left untouched.
func inheritMariadbSELinuxOptions(securityContext *corev1.PodSecurityContext, mariadb *mariadbv1alpha1.MariaDB) {
	if securityContext == nil || securityContext.SELinuxOptions != nil {
		return
	}
	if mariadb.Spec.PodSecurityContext == nil || mariadb.Spec.PodSecurityContext.SELinuxOptions == nil {
		return
	}
	securityContext.SELinuxOptions = mariadb.Spec.PodSecurityContext.SELinuxOptions
}
