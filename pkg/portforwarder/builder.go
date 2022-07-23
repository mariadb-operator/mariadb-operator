package portforwarder

import (
	"errors"
	"fmt"
	"io"

	"k8s.io/client-go/tools/portforward"
)

type Builder struct {
	kubeconfig string
	pod        string
	namespace  string
	ports      []string
	out        io.Writer
	errOut     io.Writer
}

func New() *Builder {
	return &Builder{}
}

func (b *Builder) WithKubeconfig(kubeconfig string) *Builder {
	b.kubeconfig = kubeconfig
	return b
}

func (b *Builder) WithPod(pod string) *Builder {
	b.pod = pod
	return b
}

func (b *Builder) WithNamespace(ns string) *Builder {
	b.namespace = ns
	return b
}

func (b *Builder) WithPorts(ports ...string) *Builder {
	b.ports = ports
	return b
}

func (b *Builder) WithOutputWriter(w io.Writer) *Builder {
	b.out = w
	return b
}

func (b *Builder) WithErrorWriter(w io.Writer) *Builder {
	b.errOut = w
	return b
}

func (b *Builder) Build() (*PortForwarder, error) {
	if b.pod == "" || b.namespace == "" {
		return nil, errors.New("Por and Namespace are mandatory")
	}
	if b.out == nil {
		b.out = &discardWriter{}
	}
	if b.errOut == nil {
		b.errOut = &discardWriter{}
	}

	restConfig, err := newRestConfig(b.kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error creating REST config: %v", err)
	}

	dialer, err := newDialer(restConfig, b.pod, b.namespace)
	if err != nil {
		return nil, fmt.Errorf("error creating Dialer: %v", err)
	}

	readyChan := make(chan struct{}, 1)
	stopChan := make(chan struct{}, 1)
	forwarder, err := portforward.New(*dialer, b.ports, stopChan, readyChan, b.out, b.errOut)
	if err != nil {
		return nil, fmt.Errorf("error creating Forwarder: %v", err)
	}

	return &PortForwarder{
		forwarder: forwarder,
		stopChan:  stopChan,
	}, nil
}

type discardWriter struct{}

func (w *discardWriter) Write(p []byte) (n int, err error) {
	return fmt.Fprint(io.Discard, p)
}
