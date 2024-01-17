package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	mxsclient "github.com/mariadb-operator/mariadb-operator/pkg/maxscale/client"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MaxScaleReconciler) handleAPIResult(ctx context.Context, err error) (ctrl.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}
	logger := r.apiLogger(ctx)
	logger.Error(err, "error requesting MaxScale API")
	return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
}

func (r *MaxScaleReconciler) createAdminUser(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) (ctrl.Result, error) {
	log.FromContext(ctx).V(1).Info("Creating admin user", "user", mxs.Spec.Auth.AdminUsername)

	password, err := r.RefResolver.SecretKeyRef(ctx, mxs.Spec.Auth.AdminPasswordSecretKeyRef, mxs.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting admin password: %v", err)
	}
	attrs := mxsclient.UserAttributes{
		Account:  mxsclient.UserAccountAdmin,
		Password: &password,
	}

	err = client.User.Create(ctx, mxs.Spec.Auth.AdminUsername, attrs, nil)
	return r.handleAPIResult(ctx, err)
}

func (r *MaxScaleReconciler) createServer(ctx context.Context, srv *mariadbv1alpha1.MaxScaleServer,
	client *mxsclient.Client) (ctrl.Result, error) {
	log.FromContext(ctx).V(1).Info("Creating server", "server", srv.Name)

	attrs := mxsclient.ServerAttributes{
		Parameters: mxsclient.ServerParameters{
			Address:  srv.Address,
			Port:     srv.Port,
			Protocol: srv.Protocol,
			Params:   mxsclient.NewMapParams(srv.Params),
		},
	}

	err := client.Server.Create(ctx, srv.Name, attrs, nil)
	return r.handleAPIResult(ctx, err)
}

func (r *MaxScaleReconciler) deleteServer(ctx context.Context, name string, client *mxsclient.Client) (ctrl.Result, error) {
	log.FromContext(ctx).V(1).Info("Deleting server", "server", name)

	err := client.Server.Delete(ctx, name)
	return r.handleAPIResult(ctx, err)
}

func (r *MaxScaleReconciler) createService(ctx context.Context, svc *mariadbv1alpha1.MaxScaleService, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) (ctrl.Result, error) {
	log.FromContext(ctx).V(1).Info("Creating service", "service", svc.Name)

	password, err := r.RefResolver.SecretKeyRef(ctx, mxs.Spec.Auth.ServerPasswordSecretKeyRef, mxs.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting server password: %v", err)
	}
	attrs := mxsclient.ServiceAttributes{
		Router: svc.Router,
		Parameters: mxsclient.ServiceParameters{
			User:     mxs.Spec.Auth.ServerUsername,
			Password: password,
			Params:   mxsclient.NewMapParams(svc.Params),
		},
	}
	rels := mxsclient.NewServerRelationships(mxs.ServerIDs()...)

	err = client.Service.Create(ctx, svc.Name, attrs, &rels)
	return r.handleAPIResult(ctx, err)
}

func (r *MaxScaleReconciler) createListener(ctx context.Context, svc *mariadbv1alpha1.MaxScaleService, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) (ctrl.Result, error) {
	log.FromContext(ctx).V(1).Info("Creating listener", "listener", svc.Listener.Name)

	attrs := mxsclient.ListenerAttributes{
		Parameters: mxsclient.ListenerParameters{
			Port:     svc.Listener.Port,
			Protocol: svc.Listener.Protocol,
			Params:   mxsclient.NewMapParams(svc.Listener.Params),
		},
	}
	rels := mxsclient.NewServiceRelationships(svc.Name)

	err := client.Listener.Create(ctx, svc.Listener.Name, attrs, &rels)
	return r.handleAPIResult(ctx, err)
}

func (r *MaxScaleReconciler) createMonitor(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	client *mxsclient.Client) (ctrl.Result, error) {
	log.FromContext(ctx).V(1).Info("Creating monitor", "monitor", mxs.Spec.Monitor.Name)

	password, err := r.RefResolver.SecretKeyRef(ctx, mxs.Spec.Auth.MonitorPasswordSecretKeyRef, mxs.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting monitor password: %v", err)
	}
	attrs := mxsclient.MonitorAttributes{
		Module: mxs.Spec.Monitor.Module,
		Parameters: mxsclient.MonitorParameters{
			User:            mxs.Spec.Auth.MonitorUsername,
			Password:        password,
			MonitorInterval: mxs.Spec.Monitor.Interval,
			Params:          mxsclient.NewMapParams(mxs.Spec.Monitor.Params),
		},
	}
	rels := mxsclient.NewServerRelationships(mxs.ServerIDs()...)

	err = client.Monitor.Create(ctx, mxs.Spec.Monitor.Name, attrs, &rels)
	return r.handleAPIResult(ctx, err)
}

func (r *MaxScaleReconciler) defaultClientWithPodIndex(ctx context.Context, mxs *mariadbv1alpha1.MaxScale,
	podIndex int) (*mxsclient.Client, error) {
	opts := []mdbhttp.Option{
		mdbhttp.WithTimeout(10 * time.Second),
	}
	if r.LogRequests {
		logger := r.apiLogger(ctx)
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
		logger := r.apiLogger(ctx)
		opts = append(opts, mdbhttp.WithLogger(&logger))
	}
	return mxsclient.NewClient(apiUrl, opts...)
}

func (r *MaxScaleReconciler) apiLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("api")
}
