package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/agent/errors"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/v25/pkg/http"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/pki"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	"k8s.io/utils/ptr"
)

type Client struct {
	Galera      *Galera
	Replication *Replication
}

func NewClient(baseUrl string, opts ...mdbhttp.Option) (*Client, error) {
	httpClient, err := mdbhttp.NewClient(baseUrl, opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP client: %v", err)
	}

	return &Client{
		Galera:      NewGalera(httpClient),
		Replication: NewReplication(httpClient),
	}, nil
}

func NewClientWithMariaDB(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, env *environment.OperatorEnv,
	refResolver *refresolver.RefResolver, podIndex int, opts ...mdbhttp.Option) (*Client, error) {
	baseUrl, err := getAgentBaseUrl(mariadb, podIndex)
	if err != nil {
		return nil, fmt.Errorf("error getting agent base URL: %v", err)
	}
	clientOpts, err := getClientOpts(ctx, mariadb, env, refResolver)
	if err != nil {
		return nil, fmt.Errorf("error getting client options: %v", err)
	}
	clientOpts = append(clientOpts, opts...)

	return NewClient(baseUrl, clientOpts...)
}

func handleResponse(res *http.Response, v interface{}) error {
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)

	if res.StatusCode >= 400 {
		var apiErr errors.APIError
		if err := decoder.Decode(&apiErr); err != nil {
			return fmt.Errorf("error decoding body into error: %v", err)
		}
		return errors.NewError(res.StatusCode, apiErr.Error())
	}

	if v == nil {
		return nil
	}
	if err := decoder.Decode(&v); err != nil {
		return fmt.Errorf("error decoding body: %v", err)
	}
	return nil
}

func getAgentBaseUrl(mariadb *mariadbv1alpha1.MariaDB, index int) (string, error) {
	_, agent, err := mariadb.GetDataPlaneAgent()
	if err != nil {
		return "", fmt.Errorf("error getting agent: %v", err)
	}
	scheme := "http"
	if mariadb.IsTLSEnabled() {
		scheme = "https"
	}
	return fmt.Sprintf(
		"%s://%s:%d",
		scheme,
		statefulset.PodFQDNWithService(
			mariadb.ObjectMeta,
			index,
			mariadb.InternalServiceKey().Name,
		),
		agent.Port,
	), nil
}

func getClientOpts(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, env *environment.OperatorEnv,
	refResolver *refresolver.RefResolver) ([]mdbhttp.Option, error) {
	_, agent, err := mariadb.GetDataPlaneAgent()
	if err != nil {
		return nil, fmt.Errorf("unable to get data-plane agent: %v", err)
	}
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
