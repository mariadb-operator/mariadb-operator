package controller

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/pkg/condition"
	ds "github.com/mariadb-operator/mariadb-operator/pkg/datastructures"
	mxsclient "github.com/mariadb-operator/mariadb-operator/pkg/maxscale/client"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

	if req.mxs.Status.PrimaryServer != nil && *req.mxs.Status.PrimaryServer != "" && srvStatus.primary != "" &&
		*req.mxs.Status.PrimaryServer != srvStatus.primary {
		fromServer := *req.mxs.Status.PrimaryServer
		toServer := srvStatus.primary
		log.FromContext(ctx).Info(
			"MaxScale primary server changed",
			"from-server", fromServer,
			"to-server", toServer,
		)
		r.Recorder.Event(
			req.mxs,
			corev1.EventTypeNormal,
			mariadbv1alpha1.ReasonMaxScalePrimaryServerChanged,
			fmt.Sprintf("MaxScale primary server changed from '%s' to '%s'", fromServer, toServer),
		)
	}

	monitorStatus, err = r.getMonitorStatus(ctx, req.mxs, client)
	errBundle = multierror.Append(errBundle, err)

	svcStatus, err = r.getServiceStatus(ctx, req.mxs, client)
	errBundle = multierror.Append(errBundle, err)

	listenerStatus, err = r.getListenerStatus(ctx, req.mxs, client)
	errBundle = multierror.Append(errBundle, err)

	if err := errBundle.ErrorOrNil(); err != nil {
		log.FromContext(ctx).V(1).Info("error getting status", "err", err)
	}

	return ctrl.Result{}, r.patchStatus(ctx, req.mxs, func(mss *mariadbv1alpha1.MaxScaleStatus) error {
		mss.Replicas = sts.Status.ReadyReplicas
		if srvStatus != nil {
			mss.PrimaryServer = &srvStatus.primary
			mss.Servers = srvStatus.servers
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

		condition.SetReadyWithStatefulSet(mss, &sts)
		if r.isStatefulSetReady(&sts, req.mxs) {
			condition.SetReadyWithMaxScaleStatus(mss, mss)
		}
		return nil
	})
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

func (r *MaxScaleReconciler) patchStatus(ctx context.Context, maxscale *mariadbv1alpha1.MaxScale,
	patcher func(*mariadbv1alpha1.MaxScaleStatus) error) error {
	patch := client.MergeFrom(maxscale.DeepCopy())
	if err := patcher(&maxscale.Status); err != nil {
		return err
	}
	return r.Status().Patch(ctx, maxscale, patch)
}
