package builder

import "k8s.io/apimachinery/pkg/runtime"

type Builder struct {
	scheme *runtime.Scheme
}

func New(scheme *runtime.Scheme) *Builder {
	return &Builder{
		scheme: scheme,
	}
}
