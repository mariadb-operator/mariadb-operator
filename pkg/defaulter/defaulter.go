package defaulter

import (
	corev1 "k8s.io/api/core/v1"
)

// TODO: generics.kubebuilder still does not support golang 1.18.
// See: https://github.com/kubernetes-sigs/kubebuilder/issues/2559

// TODO: mutating webhook for defaulting?

func Int32(i *int32, fallback int32) int32 {
	if i != nil {
		return *i
	}
	return fallback
}

func String(s *string, fallback string) string {
	if s != nil {
		return *s
	}
	return fallback
}

func PullPolicy(p *corev1.PullPolicy, fallback corev1.PullPolicy) corev1.PullPolicy {
	if p != nil {
		return *p
	}
	return fallback
}
