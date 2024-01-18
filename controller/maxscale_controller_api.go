package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	ds "github.com/mariadb-operator/mariadb-operator/pkg/datastructures"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	mxsclient "github.com/mariadb-operator/mariadb-operator/pkg/maxscale/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// MaxScale API

type maxScaleAPI struct {
	mxs         *mariadbv1alpha1.MaxScale
	client      *mxsclient.Client
	refResolver *refresolver.RefResolver
}

func newMaxScaleAPI(mxs *mariadbv1alpha1.MaxScale, client *mxsclient.Client, refResolver *refresolver.RefResolver) *maxScaleAPI {
	return &maxScaleAPI{
		mxs:         mxs,
		client:      client,
		refResolver: refResolver,
	}
}

func (r *maxScaleAPI) handleAPIResult(ctx context.Context, err error) (ctrl.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}
	logger := apiLogger(ctx)
	logger.Error(err, "error requesting MaxScale API")
	// TODO: emit an event?
	// TODO: update status conditions. Take into account that patching the status will trigger a reconciliation.
	return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
}

// MaxScale API - User

func (m *maxScaleAPI) createAdminUser(ctx context.Context) (ctrl.Result, error) {
	apiLogger(ctx).V(1).Info("Creating admin user", "user", m.mxs.Spec.Auth.AdminUsername)

	password, err := m.refResolver.SecretKeyRef(ctx, m.mxs.Spec.Auth.AdminPasswordSecretKeyRef, m.mxs.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting admin password: %v", err)
	}
	attrs := mxsclient.UserAttributes{
		Account:  mxsclient.UserAccountAdmin,
		Password: &password,
	}

	err = m.client.User.Create(ctx, m.mxs.Spec.Auth.AdminUsername, attrs)
	return m.handleAPIResult(ctx, err)
}

// MaxScale API - Servers

func (m *maxScaleAPI) createServer(ctx context.Context, srv *mariadbv1alpha1.MaxScaleServer) (ctrl.Result, error) {
	apiLogger(ctx).V(1).Info("Creating server", "server", srv.Name)

	err := m.client.Server.Create(ctx, srv.Name, serverAttributes(srv))
	return m.handleAPIResult(ctx, err)
}

func (m *maxScaleAPI) deleteServer(ctx context.Context, name string) (ctrl.Result, error) {
	apiLogger(ctx).V(1).Info("Deleting server", "server", name)

	err := m.client.Server.Delete(ctx, name, mxsclient.WithForceQuery())
	return m.handleAPIResult(ctx, err)
}

func (m *maxScaleAPI) patchServer(ctx context.Context, srv *mariadbv1alpha1.MaxScaleServer) (ctrl.Result, error) {
	apiLogger(ctx).V(1).Info("Patching server", "server", srv.Name)

	err := m.client.Server.Patch(ctx, srv.Name, serverAttributes(srv))
	return m.handleAPIResult(ctx, err)
}

func serverAttributes(srv *mariadbv1alpha1.MaxScaleServer) mxsclient.ServerAttributes {
	return mxsclient.ServerAttributes{
		Parameters: mxsclient.ServerParameters{
			Address:  srv.Address,
			Port:     srv.Port,
			Protocol: srv.Protocol,
			Params:   mxsclient.NewMapParams(srv.Params),
		},
	}
}

func (m *maxScaleAPI) serverRelationships(ctx context.Context) (*mxsclient.Relationships, error) {
	apiLogger(ctx).V(1).Info("Getting server relationships")

	idx, err := m.client.Server.ListIndex(ctx)
	// TODO: handleAPIResult?
	if err != nil {
		return nil, err
	}
	keys := ds.Keys(ds.Filter(idx, m.mxs.ServerIDs()...))

	return mxsclient.NewRelationshipsBuilder().
		WithServers(keys...).
		Build(), nil
}

// MaxScale API - Monitors

func (m *maxScaleAPI) createMonitor(ctx context.Context, rels *mxsclient.Relationships) (ctrl.Result, error) {
	apiLogger(ctx).V(1).Info("Creating monitor", "monitor", m.mxs.Spec.Monitor.Name)

	attrs, err := m.monitorAttributes(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting monitor attributes: %v", err)
	}

	err = m.client.Monitor.Create(ctx, m.mxs.Spec.Monitor.Name, *attrs, mxsclient.WithRelationships(rels))
	return m.handleAPIResult(ctx, err)
}

func (m *maxScaleAPI) patchMonitor(ctx context.Context, rels *mxsclient.Relationships) (ctrl.Result, error) {
	apiLogger(ctx).V(1).Info("Creating monitor", "monitor", m.mxs.Spec.Monitor.Name)

	attrs, err := m.monitorAttributes(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting monitor attributes: %v", err)
	}
	err = m.client.Monitor.Patch(ctx, m.mxs.Spec.Monitor.Name, *attrs, mxsclient.WithRelationships(rels))
	return m.handleAPIResult(ctx, err)
}

func (m *maxScaleAPI) monitorAttributes(ctx context.Context) (*mxsclient.MonitorAttributes, error) {
	password, err := m.refResolver.SecretKeyRef(ctx, m.mxs.Spec.Auth.MonitorPasswordSecretKeyRef, m.mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting monitor password: %v", err)
	}
	return &mxsclient.MonitorAttributes{
		Module: m.mxs.Spec.Monitor.Module,
		Parameters: mxsclient.MonitorParameters{
			User:            m.mxs.Spec.Auth.MonitorUsername,
			Password:        password,
			MonitorInterval: m.mxs.Spec.Monitor.Interval,
			Params:          mxsclient.NewMapParams(m.mxs.Spec.Monitor.Params),
		},
	}, nil
}

// MaxScale API - Services

func (m *maxScaleAPI) createService(ctx context.Context, svc *mariadbv1alpha1.MaxScaleService,
	rels *mxsclient.Relationships) (ctrl.Result, error) {
	apiLogger(ctx).V(1).Info("Creating service", "service", svc.Name)

	attrs, err := m.serviceAttributes(ctx, svc)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting service attributes: %v", err)
	}
	err = m.client.Service.Create(ctx, svc.Name, *attrs, mxsclient.WithRelationships(rels))
	return m.handleAPIResult(ctx, err)
}

func (m *maxScaleAPI) deleteService(ctx context.Context, name string) (ctrl.Result, error) {
	apiLogger(ctx).V(1).Info("Deleting service", "service", name)

	err := m.client.Service.Delete(ctx, name, mxsclient.WithForceQuery())
	return m.handleAPIResult(ctx, err)
}

func (m *maxScaleAPI) patchService(ctx context.Context, svc *mariadbv1alpha1.MaxScaleService,
	rels *mxsclient.Relationships) (ctrl.Result, error) {
	apiLogger(ctx).V(1).Info("Patching service", "service", svc.Name)

	attrs, err := m.serviceAttributes(ctx, svc)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting service attributes: %v", err)
	}
	err = m.client.Service.Patch(ctx, svc.Name, *attrs, mxsclient.WithRelationships(rels))
	return m.handleAPIResult(ctx, err)
}

func (m *maxScaleAPI) serviceAttributes(ctx context.Context, svc *mariadbv1alpha1.MaxScaleService) (*mxsclient.ServiceAttributes, error) {
	password, err := m.refResolver.SecretKeyRef(ctx, m.mxs.Spec.Auth.ServerPasswordSecretKeyRef, m.mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting server password: %v", err)
	}
	return &mxsclient.ServiceAttributes{
		Router: svc.Router,
		Parameters: mxsclient.ServiceParameters{
			User:     m.mxs.Spec.Auth.ServerUsername,
			Password: password,
			Params:   mxsclient.NewMapParams(svc.Params),
		},
	}, nil
}

func (m *maxScaleAPI) serviceRelationships(service string) *mxsclient.Relationships {
	return mxsclient.NewRelationshipsBuilder().
		WithServices(service).
		Build()
}

// MaxScale API - Listeners

func (m *maxScaleAPI) createListener(ctx context.Context, listener *mariadbv1alpha1.MaxScaleListener,
	rels *mxsclient.Relationships) (ctrl.Result, error) {
	apiLogger(ctx).V(1).Info("Creating listener", "listener", listener.Name)

	err := m.client.Listener.Create(ctx, listener.Name, listenerAttributes(listener), mxsclient.WithRelationships(rels))
	return m.handleAPIResult(ctx, err)
}

func (m *maxScaleAPI) deleteListener(ctx context.Context, name string) (ctrl.Result, error) {
	apiLogger(ctx).V(1).Info("Deleting listener", "listener", name)

	err := m.client.Listener.Delete(ctx, name, mxsclient.WithForceQuery())
	return m.handleAPIResult(ctx, err)
}

func (m *maxScaleAPI) patchListener(ctx context.Context, listener *mariadbv1alpha1.MaxScaleListener,
	rels *mxsclient.Relationships) (ctrl.Result, error) {
	apiLogger(ctx).V(1).Info("Patching listener", "listener", listener.Name)

	err := m.client.Listener.Patch(ctx, listener.Name, listenerAttributes(listener), mxsclient.WithRelationships(rels))
	return m.handleAPIResult(ctx, err)
}

func listenerAttributes(listener *mariadbv1alpha1.MaxScaleListener) mxsclient.ListenerAttributes {
	return mxsclient.ListenerAttributes{
		Parameters: mxsclient.ListenerParameters{
			Port:     listener.Port,
			Protocol: listener.Protocol,
			Params:   mxsclient.NewMapParams(listener.Params),
		},
	}
}

// MaxScale client

func (r *MaxScaleReconciler) defaultClientWithPodIndex(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	podIndex int) (*mxsclient.Client, error) {
	opts := []mdbhttp.Option{
		mdbhttp.WithTimeout(10 * time.Second),
	}
	if r.LogRequests {
		logger := apiLogger(ctx)
		opts = append(opts, mdbhttp.WithLogger(&logger))
	}
	return mxsclient.NewClientWithDefaultCredentials(mxs.PodAPIUrl(podIndex), opts...)
}

func (r *MaxScaleReconciler) client(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (*mxsclient.Client, error) {
	return r.clientWithAPIUrl(ctx, mxs, mxs.APIUrl())
}

func (r *MaxScaleReconciler) clientWithPodIndex(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	podIndex int) (*mxsclient.Client, error) {
	return r.clientWithAPIUrl(ctx, mxs, mxs.PodAPIUrl(podIndex))
}

func (r *MaxScaleReconciler) clientWithAPIUrl(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	apiUrl string) (*mxsclient.Client, error) {
	password, err := r.RefResolver.SecretKeyRef(ctx, mxs.Spec.Auth.AdminPasswordSecretKeyRef, mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting admin password: %v", err)
	}

	opts := []mdbhttp.Option{
		mdbhttp.WithTimeout(10 * time.Second),
		mdbhttp.WithBasicAuth(mxs.Spec.Auth.AdminUsername, password),
	}
	if r.LogRequests {
		logger := apiLogger(ctx)
		opts = append(opts, mdbhttp.WithLogger(&logger))
	}
	return mxsclient.NewClient(apiUrl, opts...)
}

func apiLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("api")
}
