package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func (b *Builder) buildContainerSecurityContext(securityContext *mariadbv1alpha1.SecurityContext) (*corev1.SecurityContext, error) {
	sccExists, err := b.discovery.SecurityContextConstrainstsExist()
	if err != nil {
		return nil, fmt.Errorf("error discovering SecurityContextConstraints: %v", err)
	}
	// Delegate SecurityContext assigment to OpenShift.
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
	sccExists, err := b.discovery.SecurityContextConstrainstsExist()
	if err != nil {
		return nil, fmt.Errorf("error discovering SecurityContextConstraints: %v", err)
	}
	// Delegate SecurityContext assigment to OpenShift.
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
	sccExists, err := b.discovery.SecurityContextConstrainstsExist()
	if err != nil {
		return nil, fmt.Errorf("error discovering SecurityContextConstraints: %v", err)
	}
	// Delegate SecurityContext assigment to OpenShift.
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
