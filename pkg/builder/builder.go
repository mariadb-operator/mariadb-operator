package builder

import (
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"k8s.io/apimachinery/pkg/runtime"
)

type Builder struct {
	scheme *runtime.Scheme
	env    *environment.Environment
}

func NewBuilder(scheme *runtime.Scheme, env *environment.Environment) *Builder {
	return &Builder{
		scheme: scheme,
		env:    env,
	}
}
