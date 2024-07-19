package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	ds "github.com/mariadb-operator/mariadb-operator/pkg/datastructures"
	mxsclient "github.com/mariadb-operator/mariadb-operator/pkg/maxscale/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/sql"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MaxScaleReconciler) reconcileStatus(ctx context.Context, req *requestMaxScale) (ctrl.Result, error) {
	var sts appsv1.StatefulSet
	if err := r.Get(ctx, client.ObjectKeyFromObject(req.mxs), &sts); err != nil {
		return ctrl.Result{}, err
	}

	client, err := r.client(ctx, req.mxs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting client: %v", err)
	}

	var (
		errBundle                 *multierror.Error
		srvStatus                 *serverStatus
		monitorStatus             *mariadbv1alpha1.MaxScaleResourceStatus
		svcStatus, listenerStatus []mariadbv1alpha1.MaxScaleResourceStatus
	)

	srvStatus, err = r.getServerStatus(ctx, req.mxs, client)
	errBundle = multierror.Append(errBundle, err)

	currentPrimary := ptr.Deref(req.mxs.Status.PrimaryServer, "")
	newPrimary := ptr.Deref(srvStatus, serverStatus{}).primary

	if currentPrimary != "" && newPrimary != "" && currentPrimary != newPrimary {
		log.FromContext(ctx).Info(
			"MaxScale primary server changed",
			"from-server", currentPrimary,
			"to-server", newPrimary,
		)
		r.Recorder.Event(
			req.mxs,
			corev1.EventTypeNormal,
			mariadbv1alpha1.ReasonMaxScalePrimaryServerChanged,
			fmt.Sprintf("MaxScale primary server changed from '%s' to '%s'", currentPrimary, newPrimary),
		)
	}

	monitorStatus, err = r.getMonitorStatus(ctx, req.mxs, client)
	errBundle = multierror.Append(errBundle, err)

	svcStatus, err = r.getServiceStatus(ctx, req.mxs, client)
	errBundle = multierror.Append(errBundle, err)

	listenerStatus, err = r.getListenerStatus(ctx, req.mxs, client)
	errBundle = multierror.Append(errBundle, err)

	configSync, err := r.getConfigSyncStatus(ctx, req.mxs, client)
	errBundle = multierror.Append(errBundle, err)

	if err := errBundle.ErrorOrNil(); err != nil {
		log.FromContext(ctx).V(1).Info("error getting status", "err", err)
	}

	return ctrl.Result{}, r.patchStatus(ctx, req.mxs, func(mss *mariadbv1alpha1.MaxScaleStatus) error {
		mss.Replicas = sts.Status.ReadyReplicas
		if srvStatus != nil {
			if srvStatus.primary != "" {
				mss.PrimaryServer = &srvStatus.primary
			}
			if srvStatus.servers != nil {
				mss.Servers = srvStatus.servers
			}
		}
		if monitorStatus != nil {
			mss.Monitor = monitorStatus
		}
		if svcStatus != nil {
			mss.Services = svcStatus
		}
		if listenerStatus != nil {
			mss.Listeners = listenerStatus
		}
		if configSync != nil {
			mss.ConfigSync = configSync
		}

		condition.SetReadyWithStatefulSet(mss, &sts)
		if r.isStatefulSetReady(&sts, req.mxs) {
			condition.SetReadyWithMaxScaleStatus(mss, mss)
		}
		return nil
	})
}

func (r *MaxScaleReconciler) handleConfigSyncConflict(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	err error) error {
	if err == nil || !mxs.IsHAEnabled() {
		return nil
	}

	configSync := ptr.Deref(mxs.Status.ConfigSync, mariadbv1alpha1.MaxScaleConfigSyncStatus{})
	if configSync.MaxScaleVersion <= configSync.DatabaseVersion {
		return nil
	}
	log.FromContext(ctx).Info(
		"Config sync conflict detected",
		"maxscale-version", configSync.MaxScaleVersion,
		"database-version", configSync.DatabaseVersion,
	)

	client, err := r.getPrimarySqlClient(ctx, mxs)
	if err != nil {
		return fmt.Errorf("error getting primary SQL client: %v", err)
	}
	if err := client.TruncateMaxScaleConfig(ctx); err != nil {
		return fmt.Errorf("error truncating maxscale_config table: %v", err)
	}
	return nil
}

type serverStatus struct {
	primary string
	servers []mariadbv1alpha1.MaxScaleServerStatus
}

func (r *MaxScaleReconciler) getServerStatus(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) (*serverStatus, error) {
	serverIdx, err := client.Server.ListIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting servers: %v", err)
	}
	serverIdx = ds.Filter(serverIdx, mxs.ServerIDs()...)

	serverStatuses := make([]mariadbv1alpha1.MaxScaleServerStatus, len(serverIdx))
	i := 0
	for _, srv := range serverIdx {
		serverStatuses[i] = mariadbv1alpha1.MaxScaleServerStatus{
			Name:  srv.ID,
			State: srv.Attributes.State,
		}
		i++
	}
	var primary string
	for _, srv := range serverIdx {
		if srv.Attributes.IsMaster() {
			primary = srv.ID
			break
		}
	}

	return &serverStatus{
		primary: primary,
		servers: serverStatuses,
	}, nil
}

func (r *MaxScaleReconciler) getMonitorStatus(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) (*mariadbv1alpha1.MaxScaleResourceStatus, error) {
	monitor, err := client.Monitor.Get(ctx, mxs.Spec.Monitor.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting monitor: %v", err)
	}
	return &mariadbv1alpha1.MaxScaleResourceStatus{
		Name:  monitor.ID,
		State: monitor.Attributes.State,
	}, nil
}

func (r *MaxScaleReconciler) getServiceStatus(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) ([]mariadbv1alpha1.MaxScaleResourceStatus, error) {
	serviceIdx, err := client.Service.ListIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting services: %v", err)
	}
	serviceIdx = ds.Filter(serviceIdx, mxs.ServiceIDs()...)

	serviceStatuses := make([]mariadbv1alpha1.MaxScaleResourceStatus, len(serviceIdx))
	i := 0
	for _, svc := range serviceIdx {
		serviceStatuses[i] = mariadbv1alpha1.MaxScaleResourceStatus{
			Name:  svc.ID,
			State: svc.Attributes.State,
		}
		i++
	}
	return serviceStatuses, nil
}

func (r *MaxScaleReconciler) getListenerStatus(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) ([]mariadbv1alpha1.MaxScaleResourceStatus, error) {
	listenerIdx, err := client.Listener.ListIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting listeners: %v", err)
	}
	listenerIdx = ds.Filter(listenerIdx, mxs.ListenerIDs()...)

	listenerStatuses := make([]mariadbv1alpha1.MaxScaleResourceStatus, len(listenerIdx))
	i := 0
	for _, listener := range listenerIdx {
		listenerStatuses[i] = mariadbv1alpha1.MaxScaleResourceStatus{
			Name:  listener.ID,
			State: listener.Attributes.State,
		}
		i++
	}
	return listenerStatuses, nil
}

func (r *MaxScaleReconciler) getConfigSyncStatus(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) (*mariadbv1alpha1.MaxScaleConfigSyncStatus, error) {
	if !mxs.IsHAEnabled() {
		return nil, nil
	}

	var errBundle *multierror.Error
	mxsVersion, err := r.getMaxScaleConfigSyncVersion(ctx, client)
	errBundle = multierror.Append(errBundle, err)

	dbVersion, err := r.getDatabaseConfigSyncVersion(ctx, mxs)
	errBundle = multierror.Append(errBundle, err)

	if err := errBundle.ErrorOrNil(); err != nil {
		return nil, err
	}

	return &mariadbv1alpha1.MaxScaleConfigSyncStatus{
		MaxScaleVersion: mxsVersion,
		DatabaseVersion: dbVersion,
	}, nil
}

func (r *MaxScaleReconciler) getMaxScaleConfigSyncVersion(ctx context.Context, client *mxsclient.Client) (int, error) {
	mxsStatus, err := client.MaxScale.Get(ctx)
	if err != nil {
		return 0, fmt.Errorf("error getting MaxScale status: %v", err)
	}
	if mxsStatus.Attributes.ConfigSync == nil {
		return 0, errors.New("MaxScale config sync not set")
	}
	return mxsStatus.Attributes.ConfigSync.Version, nil
}

func (r *MaxScaleReconciler) getDatabaseConfigSyncVersion(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (int, error) {
	client, err := r.getReadySqlClient(ctx, mxs)
	if err != nil {
		return 0, fmt.Errorf("error getting primary SQL client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.FromContext(ctx).Error(err, "error closing SQL connection")
		}
	}()
	return client.MaxScaleConfigSyncVersion(ctx)
}

func (r *MaxScaleReconciler) getPrimarySqlClient(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (*sql.Client, error) {
	primaryName := mxs.Status.GetPrimaryServer()
	if primaryName == nil {
		return nil, errors.New("primary server not found in status")
	}
	return r.getSqlClient(ctx, mxs, *primaryName)
}

func (r *MaxScaleReconciler) getReadySqlClient(ctx context.Context, mxs *mariadbv1alpha1.MaxScale) (*sql.Client, error) {
	var readyServer string
	for _, srv := range mxs.Status.Servers {
		if srv.IsReady() {
			readyServer = srv.Name
			break
		}
	}
	if readyServer == "" {
		return nil, errors.New("ready server not found in status")
	}
	return r.getSqlClient(ctx, mxs, readyServer)
}

func (r *MaxScaleReconciler) getSqlClient(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	serverName string) (*sql.Client, error) {
	if mxs.Spec.Config.Sync == nil {
		return nil, errors.New("config sync must be enabled")
	}
	if mxs.Spec.Auth.SyncUsername == nil || mxs.Spec.Auth.SyncPasswordSecretKeyRef == nil {
		return nil, errors.New("Config sync credentials must be set")
	}

	serverIdx := mxs.ServerIndex()
	srv, ok := serverIdx[serverName]
	if !ok {
		return nil, errors.New("primary server not found in spec")
	}

	password, err := r.RefResolver.SecretKeyRef(ctx, mxs.Spec.Auth.SyncPasswordSecretKeyRef.SecretKeySelector, mxs.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting sync password: %v", err)
	}

	return sql.NewClient(
		sql.WitHost(srv.Address),
		sql.WithPort(srv.Port),
		sql.WithDatabase(mxs.Spec.Config.Sync.Database),
		sql.WithUsername(*mxs.Spec.Auth.SyncUsername),
		sql.WithPassword(password),
	)
}

func (r *MaxScaleReconciler) patchStatus(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale,
	patcher func(*mariadbv1alpha1.MaxScaleStatus) error) error {
	patch := client.MergeFrom(maxscale.DeepCopy())
	if err := patcher(&maxscale.Status); err != nil {
		return err
	}
	return r.Status().Patch(ctx, maxscale, patch)
}
