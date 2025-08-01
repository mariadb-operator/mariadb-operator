package builder

import (
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"k8s.io/apimachinery/pkg/runtime"
)

type Builder struct {
	scheme    *runtime.Scheme
	env       *environment.OperatorEnv
	discovery *discovery.Discovery
}

func NewBuilder(scheme *runtime.Scheme, env *environment.OperatorEnv, discovery *discovery.Discovery) *Builder {
	return &Builder{
		scheme:    scheme,
		env:       env,
		discovery: discovery,
	}
}
