package builder

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func (b *Builder) buildContainerSecurityContext(securityContext *corev1.SecurityContext) (*corev1.SecurityContext, error) {
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
	return securityContext, nil
}
