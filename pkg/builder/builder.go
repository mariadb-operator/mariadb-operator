package builder

import (
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"k8s.io/apimachinery/pkg/runtime"
)

type Builder struct {
	scheme *runtime.Scheme
	env    *environment.OperatorEnv
}

func NewBuilder(scheme *runtime.Scheme, env *environment.OperatorEnv) *Builder {
	return &Builder{
		scheme: scheme,
		env:    env,
	}
}
