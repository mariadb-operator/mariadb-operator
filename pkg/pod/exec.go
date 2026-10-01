package pod

import (
	"bytes"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// Executor runs a command inside a container of a running Pod.
type Executor interface {
	Exec(ctx context.Context, pod *corev1.Pod, container string, command []string) error
}

// RemoteExecutor runs commands through the Kubernetes API Pod exec subresource.
type RemoteExecutor struct {
	config    *rest.Config
	clientset kubernetes.Interface
}

func NewRemoteExecutor(config *rest.Config, clientset kubernetes.Interface) *RemoteExecutor {
	return &RemoteExecutor{
		config:    config,
		clientset: clientset,
	}
}

// Exec runs command in the given container and returns an error carrying stderr when it does not succeed.
func (e *RemoteExecutor) Exec(ctx context.Context, pod *corev1.Pod, container string, command []string) error {
	req := e.clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	wsExec, err := remotecommand.NewWebSocketExecutor(e.config, "GET", req.URL().String())
	if err != nil {
		return fmt.Errorf("error creating WebSocket executor: %v", err)
	}
	spdyExec, err := remotecommand.NewSPDYExecutor(e.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("error creating SPDY executor: %v", err)
	}
	exec, err := remotecommand.NewFallbackExecutor(wsExec, spdyExec, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
	if err != nil {
		return fmt.Errorf("error creating executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		return fmt.Errorf("error executing %v in Pod '%s': %v (stderr: %q)", command, pod.Name, err, stderr.String())
	}
	return nil
}
