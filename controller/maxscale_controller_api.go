package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	ds "github.com/mariadb-operator/mariadb-operator/pkg/datastructures"
	"github.com/mariadb-operator/mariadb-operator/pkg/health"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	mxsclient "github.com/mariadb-operator/mariadb-operator/pkg/maxscale/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/refresolver"
	"k8s.io/utils/ptr"
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

// MaxScale API - User

func (m *maxScaleAPI) createAdminUser(ctx context.Context, username, password string) error {
	attrs := mxsclient.UserAttributes{
		Account:  mxsclient.UserAccountAdmin,
		Password: &password,
	}
	return m.client.User.Create(ctx, username, attrs)
}

func (m *maxScaleAPI) patchUser(ctx context.Context, username, password string) error {
	attrs := mxsclient.UserAttributes{
		Password: &password,
	}
	return m.client.User.Patch(ctx, username, attrs)
}

// MaxScale API - Servers

func (m *maxScaleAPI) createServer(ctx context.Context, srv *mariadbv1alpha1.MaxScaleServer) error {
	return m.client.Server.Create(ctx, srv.Name, serverAttributes(srv))
}

func (m *maxScaleAPI) deleteServer(ctx context.Context, name string) error {
	return m.client.Server.Delete(ctx, name, mxsclient.WithForceQuery())
}

func (m *maxScaleAPI) patchServer(ctx context.Context, srv *mariadbv1alpha1.MaxScaleServer) error {
	return m.client.Server.Patch(ctx, srv.Name, serverAttributes(srv))
}

func (m *maxScaleAPI) updateServerState(ctx context.Context, srv *mariadbv1alpha1.MaxScaleServer) error {
	if srv.Maintenance {
		return m.client.Server.SetMaintenance(ctx, srv.Name)
	}
	return m.client.Server.ClearMaintenance(ctx, srv.Name)
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
	idx, err := m.client.Server.ListIndex(ctx)
	if err != nil {
		return nil, err
	}
	keys := ds.Keys(ds.Filter(idx, m.mxs.ServerIDs()...))
	sort.Strings(keys)

	return mxsclient.NewRelationshipsBuilder().
		WithServers(keys...).
		Build(), nil
}

// MaxScale API - Monitors

func (m *maxScaleAPI) createMonitor(ctx context.Context, rels *mxsclient.Relationships) error {
	attrs, err := m.monitorAttributes(ctx)
	if err != nil {
		return fmt.Errorf("error getting monitor attributes: %v", err)
	}

	return m.client.Monitor.Create(ctx, m.mxs.Spec.Monitor.Name, *attrs, mxsclient.WithRelationships(rels))
}

func (m *maxScaleAPI) patchMonitor(ctx context.Context, rels *mxsclient.Relationships) error {
	attrs, err := m.monitorAttributes(ctx)
	if err != nil {
		return fmt.Errorf("error getting monitor attributes: %v", err)
	}
	return m.client.Monitor.Patch(ctx, m.mxs.Spec.Monitor.Name, *attrs, mxsclient.WithRelationships(rels))
}

func (m *maxScaleAPI) updateMonitorState(ctx context.Context) error {
	if m.mxs.Spec.Monitor.Suspend {
		return m.client.Monitor.Stop(ctx, m.mxs.Spec.Monitor.Name)
	}
	return m.client.Monitor.Start(ctx, m.mxs.Spec.Monitor.Name)
}

func (m *maxScaleAPI) monitorAttributes(ctx context.Context) (*mxsclient.MonitorAttributes, error) {
	password, err := m.refResolver.SecretKeyRef(ctx, m.mxs.Spec.Auth.MonitorPasswordSecretKeyRef.SecretKeySelector, m.mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting monitor password: %v", err)
	}
	attrs := &mxsclient.MonitorAttributes{
		Module: m.mxs.Spec.Monitor.Module,
		Parameters: mxsclient.MonitorParameters{
			User:            m.mxs.Spec.Auth.MonitorUsername,
			Password:        password,
			MonitorInterval: m.mxs.Spec.Monitor.Interval,
			Params:          mxsclient.NewMapParams(m.mxs.Spec.Monitor.Params),
		},
	}
	if m.mxs.IsHAEnabled() && m.mxs.Spec.Monitor.Module == mariadbv1alpha1.MonitorModuleMariadb {
		if m.mxs.Spec.Monitor.CooperativeMonitoring != nil {
			attrs.Parameters.CooperativeMonitoringLocks = m.mxs.Spec.Monitor.CooperativeMonitoring
		} else {
			attrs.Parameters.CooperativeMonitoringLocks = ptr.To(mariadbv1alpha1.CooperativeMonitoringMajorityOfAll)
		}
	}
	return attrs, nil
}

// MaxScale API - Services

func (m *maxScaleAPI) createService(ctx context.Context, svc *mariadbv1alpha1.MaxScaleService, rels *mxsclient.Relationships) error {
	attrs, err := m.serviceAttributes(ctx, svc)
	if err != nil {
		return fmt.Errorf("error getting service attributes: %v", err)
	}
	return m.client.Service.Create(ctx, svc.Name, *attrs, mxsclient.WithRelationships(rels))
}

func (m *maxScaleAPI) deleteService(ctx context.Context, name string) error {
	return m.client.Service.Delete(ctx, name, mxsclient.WithForceQuery())
}

func (m *maxScaleAPI) patchService(ctx context.Context, svc *mariadbv1alpha1.MaxScaleService, rels *mxsclient.Relationships) error {
	attrs, err := m.serviceAttributes(ctx, svc)
	if err != nil {
		return fmt.Errorf("error getting service attributes: %v", err)
	}
	return m.client.Service.Patch(ctx, svc.Name, *attrs, mxsclient.WithRelationships(rels))
}

func (m *maxScaleAPI) updateServiceState(ctx context.Context, svc *mariadbv1alpha1.MaxScaleService) error {
	if svc.Suspend {
		return m.client.Service.Stop(ctx, svc.Name)
	}
	return m.client.Service.Start(ctx, svc.Name)
}

func (m *maxScaleAPI) serviceAttributes(ctx context.Context, svc *mariadbv1alpha1.MaxScaleService) (*mxsclient.ServiceAttributes, error) {
	password, err := m.refResolver.SecretKeyRef(ctx, m.mxs.Spec.Auth.ServerPasswordSecretKeyRef.SecretKeySelector, m.mxs.Namespace)
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

func (m *maxScaleAPI) createListener(ctx context.Context, listener *mariadbv1alpha1.MaxScaleListener, rels *mxsclient.Relationships) error {
	return m.client.Listener.Create(ctx, listener.Name, listenerAttributes(listener), mxsclient.WithRelationships(rels))
}

func (m *maxScaleAPI) deleteListener(ctx context.Context, name string) error {
	return m.client.Listener.Delete(ctx, name, mxsclient.WithForceQuery())
}

func (m *maxScaleAPI) patchListener(ctx context.Context, listener *mariadbv1alpha1.MaxScaleListener, rels *mxsclient.Relationships) error {
	return m.client.Listener.Patch(ctx, listener.Name, listenerAttributes(listener), mxsclient.WithRelationships(rels))
}

func (m *maxScaleAPI) updateListenerState(ctx context.Context, listener *mariadbv1alpha1.MaxScaleListener) error {
	if listener.Suspend {
		return m.client.Listener.Stop(ctx, listener.Name)
	}
	return m.client.Listener.Start(ctx, listener.Name)
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

// MaxScale API - MaxScale

func (m *maxScaleAPI) isMaxScaleConfigSynced(ctx context.Context) (bool, error) {
	data, err := m.client.MaxScale.Get(ctx)
	if err != nil {
		return false, err
	}
	params := data.Attributes.Parameters

	return params.ConfigSyncCluster == m.mxs.Spec.Monitor.Name &&
		params.ConfigSyncUser == ptr.Deref(m.mxs.Spec.Auth.SyncUsername, "") &&
		params.ConfigSyncDB == m.mxs.Spec.Config.Sync.Database, nil
}

func (m *maxScaleAPI) patchMaxScaleConfigSync(ctx context.Context) error {
	if m.mxs.Spec.Config.Sync == nil {
		return errors.New("'spec.config.sync' must be set")
	}
	if m.mxs.Spec.Auth.SyncUsername == nil || m.mxs.Spec.Auth.SyncPasswordSecretKeyRef == nil {
		return errors.New("'Config sync credentials must be set")
	}
	password, err := m.refResolver.SecretKeyRef(ctx, m.mxs.Spec.Auth.SyncPasswordSecretKeyRef.SecretKeySelector, m.mxs.Namespace)
	if err != nil {
		return fmt.Errorf("error getting sync password: %v", err)
	}
	attrs := mxsclient.MaxScaleAttributes{
		Parameters: mxsclient.MaxScaleParameters{
			ConfigSyncCluster:  m.mxs.Spec.Monitor.Name,
			ConfigSyncUser:     *m.mxs.Spec.Auth.SyncUsername,
			ConfigSyncPassword: password,
			ConfigSyncDB:       m.mxs.Spec.Config.Sync.Database,
			ConfigSyncInterval: m.mxs.Spec.Config.Sync.Interval,
			ConfigSyncTimeout:  m.mxs.Spec.Config.Sync.Timeout,
		},
	}

	return m.client.MaxScale.Patch(ctx, attrs)
}

// MaxScale client

func (r *MaxScaleReconciler) defaultClientWithPodIndex(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	podIndex int) (*mxsclient.Client, error) {
	opts := []mdbhttp.Option{
		mdbhttp.WithTimeout(10 * time.Second),
	}
	if r.LogMaxScale {
		logger := apiLogger(ctx)
		opts = append(opts, mdbhttp.WithLogger(&logger))
	}
	return mxsclient.NewClientWithDefaultCredentials(mxs.PodAPIUrl(podIndex), opts...)
}

func (r *MaxScaleReconciler) client(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (*mxsclient.Client, error) {
	return r.clientWithAPIUrl(ctx, mxs, mxs.APIUrl())
}

func (r *MaxScaleReconciler) clientWitHealthyPod(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (*mxsclient.Client, error) {
	podIndex, err := health.HealthyMaxScalePod(ctx, r.Client, mxs)
	if err != nil {
		return nil, fmt.Errorf("error getting healthy Pod: %v", err)
	}
	return r.clientWithPodIndex(ctx, mxs, *podIndex)
}

func (r *MaxScaleReconciler) clientWithPodIndex(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	podIndex int) (*mxsclient.Client, error) {
	return r.clientWithAPIUrl(ctx, mxs, mxs.PodAPIUrl(podIndex))
}

func (r *MaxScaleReconciler) clientWithAPIUrl(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	apiUrl string) (*mxsclient.Client, error) {
	password, err := r.RefResolver.SecretKeyRef(ctx, mxs.Spec.Auth.AdminPasswordSecretKeyRef.SecretKeySelector, mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting admin password: %v", err)
	}

	opts := []mdbhttp.Option{
		mdbhttp.WithTimeout(10 * time.Second),
		mdbhttp.WithBasicAuth(mxs.Spec.Auth.AdminUsername, password),
	}
	if r.LogMaxScale {
		logger := apiLogger(ctx)
		opts = append(opts, mdbhttp.WithLogger(&logger))
	}
	return mxsclient.NewClient(apiUrl, opts...)
}

func apiLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("api")
}
