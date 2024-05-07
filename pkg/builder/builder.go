package builder

import (
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"k8s.io/apimachinery/pkg/runtime"
)

type Builder struct {
	scheme    *runtime.Scheme
	env       *environment.OperatorEnv
	discovery *discovery.Discovery
}

type BuilderOption func(b *Builder)

func WithDiscovery(d *discovery.Discovery) BuilderOption {
	return func(b *Builder) {
		b.discovery = d
	}
}

func NewBuilder(scheme *runtime.Scheme, env *environment.OperatorEnv, opts ...BuilderOption) (*Builder, error) {
	discovery, err := discovery.NewDiscovery()
	if err != nil {
		return nil, fmt.Errorf("error creating discovery: %v", err)
	}
	builder := &Builder{
		scheme:    scheme,
		env:       env,
		discovery: discovery,
	}
	for _, setOpt := range opts {
		setOpt(builder)
	}
	return builder, nil
}
