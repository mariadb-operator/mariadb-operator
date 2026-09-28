package clientset

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v26/pkg/http"
	mxsclient "github.com/mariadb-operator/mariadb-operator/v26/pkg/maxscale/client"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/pki"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Option func(*ClientSet)

// WithAPILogging enables logging of the requests sent to the MaxScale API.
func WithAPILogging(logAPI bool) Option {
	return func(c *ClientSet) {
		c.logAPI = logAPI
	}
}

// ClientSet builds authenticated clients for the MaxScale API.
type ClientSet struct {
	mxs         *mariadbv1alpha1.MaxScale
	refResolver *refresolver.RefResolver
	logAPI      bool
}

func NewClientSet(mxs *mariadbv1alpha1.MaxScale, refResolver *refresolver.RefResolver, opts ...Option) *ClientSet {
	c := &ClientSet{
		mxs:         mxs,
		refResolver: refResolver,
	}
	for _, setOpt := range opts {
		setOpt(c)
	}
	return c
}

// Client returns a client for the MaxScale API Service.
func (c *ClientSet) Client(ctx context.Context) (*mxsclient.Client, error) {
	return c.ClientWithAPIUrl(ctx, c.mxs.APIUrl())
}

// ClientSetByPod returns a client for each MaxScale Pod, indexed by Pod name.
func (c *ClientSet) ClientSetByPod(ctx context.Context) (map[string]*mxsclient.Client, error) {
	clientSet := make(map[string]*mxsclient.Client, c.mxs.Spec.Replicas)
	for i := 0; i < int(c.mxs.Spec.Replicas); i++ {
		pod := stsobj.PodName(c.mxs.ObjectMeta, i)

		client, err := c.ClientWithAPIUrl(ctx, c.mxs.PodAPIUrl(i))
		if err != nil {
			return nil, fmt.Errorf("error getting client for Pod '%s': %w", pod, err)
		}
		clientSet[pod] = client
	}
	return clientSet, nil
}

// PrimaryClient returns the primary MaxScale client.
func (c *ClientSet) PrimaryClient(ctx context.Context) (*mxsclient.Client, error) {
	if c.mxs.Spec.Monitor.Module != mariadbv1alpha1.MonitorModuleMariadb || c.mxs.Spec.Monitor.Name == "" {
		return nil, nil
	}
	logger := log.FromContext(ctx).WithName("primary-client").V(1)

	for i := 0; i < int(c.mxs.Spec.Replicas); i++ {
		client, err := c.ClientWithPodIndex(ctx, i)
		if err != nil {
			return nil, fmt.Errorf("error getting client for Pod index %d: %v", i, err)
		}
		monitor, err := client.Monitor.Get(ctx, c.mxs.Spec.Monitor.Name)
		if err != nil {
			logger.Info("error getting monitor", "monitor", c.mxs.Spec.Monitor.Name, "err", err)
			continue
		}
		if monitor.Attributes != nil && monitor.Attributes.Diagnostics != nil &&
			ptr.Deref(monitor.Attributes.Diagnostics.Primary, false) {
			return client, nil
		}
	}
	return nil, nil
}

// ClientWithPodIndex returns a client for the MaxScale Pod with the given index.
func (c *ClientSet) ClientWithPodIndex(ctx context.Context, podIndex int) (*mxsclient.Client, error) {
	return c.ClientWithAPIUrl(ctx, c.mxs.PodAPIUrl(podIndex))
}

// ClientWithAPIUrl returns a client for the given MaxScale API URL, authenticated with the admin credentials.
func (c *ClientSet) ClientWithAPIUrl(ctx context.Context, apiUrl string) (*mxsclient.Client, error) {
	password, err := c.refResolver.SecretKeyRef(ctx, c.mxs.Spec.Auth.AdminPasswordSecretKeyRef.SecretKeySelector, c.mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting admin password: %v", err)
	}

	opts := []mdbhttp.Option{
		mdbhttp.WithTimeout(10 * time.Second),
		mdbhttp.WithBasicAuth(c.mxs.Spec.Auth.AdminUsername, password),
	}
	if c.logAPI {
		logger := APILogger(ctx)
		opts = append(opts, mdbhttp.WithLogger(&logger))
	}
	if c.mxs.IsTLSEnabled() {
		tlsOpts, err := c.TLSOptions(ctx)
		if err != nil {
			return nil, fmt.Errorf("error getting client TLS options: %v", err)
		}
		opts = append(opts, tlsOpts...)
	}
	return mxsclient.NewClient(apiUrl, opts...)
}

// TLSOptions returns the TLS options of the MaxScale API clients: CA bundle and admin client certificate.
func (c *ClientSet) TLSOptions(ctx context.Context) ([]mdbhttp.Option, error) {
	if !c.mxs.IsTLSEnabled() {
		return nil, nil
	}
	tlsCA, err := c.refResolver.SecretKeyRef(ctx, c.mxs.TLSCABundleSecretKeyRef(), c.mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading TLS CA bundle: %v", err)
	}

	adminCertKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: c.mxs.TLSAdminCertSecretKey().Name,
		},
		Key: pki.TLSCertKey,
	}
	tlsCert, err := c.refResolver.SecretKeyRef(ctx, adminCertKeySelector, c.mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading TLS cert: %v", err)
	}

	adminKeyKeySelector := mariadbv1alpha1.SecretKeySelector{
		LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
			Name: c.mxs.TLSAdminCertSecretKey().Name,
		},
		Key: pki.TLSKeyKey,
	}
	tlsKey, err := c.refResolver.SecretKeyRef(ctx, adminKeyKeySelector, c.mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error reading TLS cert: %v", err)
	}

	return []mdbhttp.Option{
		mdbhttp.WithTLSEnabled(c.mxs.IsTLSEnabled()),
		mdbhttp.WithTLSCA([]byte(tlsCA)),
		mdbhttp.WithTLSCert([]byte(tlsCert)),
		mdbhttp.WithTLSKey([]byte(tlsKey)),
	}, nil
}

// APILogger returns the logger used for the MaxScale API requests.
func APILogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("api")
}
