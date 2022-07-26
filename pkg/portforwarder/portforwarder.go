package portforwarder

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/client-go/util/homedir"
)

type PortForwarder struct {
	forwarder *portforward.PortForwarder
	stopChan  chan struct{}
}

func (pf *PortForwarder) Run(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		close(pf.stopChan)
	}()

	return retry.Do(
		func() error {
			if err := pf.forwarder.ForwardPorts(); err != nil {
				return fmt.Errorf("error forwarding ports: %v", err)
			}
			return nil
		},
		retry.Context(ctx),
		retry.Attempts(10),
		retry.Delay(1*time.Second),
		retry.DelayType(retry.BackOffDelay),
	)
}

func newRestConfig(customKubeconfig string) (*rest.Config, error) {
	var kubeconfig string
	if customKubeconfig != "" {
		kubeconfig = customKubeconfig
	} else if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	} else {
		return nil, errors.New("unable to find kubeconfig file")
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

func newDialer(config *rest.Config, pod, namespace string) (*httpstream.Dialer, error) {
	roundTripper, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return nil, fmt.Errorf("error creating rount tripper: %v", err)
	}
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, pod)
	hostIP := strings.TrimLeft(config.Host, "htps:/")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)
	return &dialer, nil
}
