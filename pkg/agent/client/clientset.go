package client

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/pki"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	"k8s.io/utils/ptr"
)

type ClientSet struct {
	mariadb       *mariadbv1alpha1.MariaDB
	clientOpts    []mdbhttp.Option
	clientByIndex map[int]*Client
	mux           *sync.Mutex
}

func NewClientSet(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, env *environment.OperatorEnv,
	refResolver *refresolver.RefResolver, opts ...mdbhttp.Option) (*ClientSet, error) {
	if !mariadb.IsHAEnabled() {
		return nil, errors.New("HA should be enabled to create an agent ClientSet")
	}
	_, agent, err := mariadb.GetDataPlaneAgent()
	if err != nil {
		return nil, fmt.Errorf("unable to get data-plane agent: %v", err)
	}

	clientOpts, err := getClientOpts(ctx, mariadb, agent, env, refResolver)
	if err != nil {
		return nil, fmt.Errorf("error getting client options: %v", err)
	}
	clientOpts = append(clientOpts, opts...)

	return &ClientSet{
		mariadb:       mariadb,
		clientOpts:    clientOpts,
		clientByIndex: make(map[int]*Client),
		mux:           &sync.Mutex{},
	}, nil
}

func (c *ClientSet) ClientForIndex(index int) (*Client, error) {
	if err := c.validateIndex(index); err != nil {
		return nil, fmt.Errorf("invalid index: %v", err)
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	if client, ok := c.clientByIndex[index]; ok {
		return client, nil
	}

	baseUrl, err := getAgentBaseUrl(c.mariadb, index)
	if err != nil {
		return nil, fmt.Errorf("error getting base URL: %v", err)
	}
	client, err := NewClient(baseUrl, c.clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("error creating client: %v", err)
	}
	c.clientByIndex[index] = client

	return client, nil
}

func (c *ClientSet) validateIndex(index int) error {
	if index >= 0 && index < int(c.mariadb.Spec.Replicas) {
		return nil
	}
	return fmt.Errorf("index '%d' out of MariaDB replicas bounds [0, %d]", index, c.mariadb.Spec.Replicas-1)
}

func getClientOpts(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, agent *mariadbv1alpha1.Agent,
	env *environment.OperatorEnv, refResolver *refresolver.RefResolver) ([]mdbhttp.Option, error) {
	opts := []mdbhttp.Option{}
	kubernetesAuth := ptr.Deref(agent.KubernetesAuth, mariadbv1alpha1.KubernetesAuth{})
	basicAuth := ptr.Deref(agent.BasicAuth, mariadbv1alpha1.BasicAuth{})

	if kubernetesAuth.Enabled {
		opts = append(opts,
			mdbhttp.WithKubernetesAuth(env.MariadbOperatorSAPath),
		)
	} else if basicAuth.Enabled && !reflect.ValueOf(basicAuth.PasswordSecretKeyRef).IsZero() {
		password, err := refResolver.SecretKeyRef(ctx, basicAuth.PasswordSecretKeyRef.SecretKeySelector, mariadb.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error getting agent password: %v", err)
		}
		opts = append(opts,
			mdbhttp.WithBasicAuth(basicAuth.Username, password),
		)
	}

	if mariadb.IsTLSEnabled() {
		tlsCA, err := refResolver.SecretKeyRef(ctx, mariadb.TLSCABundleSecretKeyRef(), mariadb.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error reading TLS CA bundle: %v", err)
		}

		opts = append(opts, []mdbhttp.Option{
			mdbhttp.WithTLSEnabled(mariadb.IsTLSEnabled()),
			mdbhttp.WithTLSCA([]byte(tlsCA)),
		}...)

		if mariadb.IsTLSMutual() {
			clientCertKeySelector := mariadbv1alpha1.SecretKeySelector{
				LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
					Name: mariadb.TLSClientCertSecretKey().Name,
				},
				Key: pki.TLSCertKey,
			}
			tlsCert, err := refResolver.SecretKeyRef(ctx, clientCertKeySelector, mariadb.Namespace)
			if err != nil {
				return nil, fmt.Errorf("error reading TLS cert: %v", err)
			}

			clientKeyKeySelector := mariadbv1alpha1.SecretKeySelector{
				LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
					Name: mariadb.TLSClientCertSecretKey().Name,
				},
				Key: pki.TLSKeyKey,
			}
			tlsKey, err := refResolver.SecretKeyRef(ctx, clientKeyKeySelector, mariadb.Namespace)
			if err != nil {
				return nil, fmt.Errorf("error reading TLS key: %v", err)
			}

			opts = append(opts, []mdbhttp.Option{
				mdbhttp.WithTLSCert([]byte(tlsCert)),
				mdbhttp.WithTLSKey([]byte(tlsKey)),
			}...)
		}
	}

	return opts, nil
}
