package v1alpha1

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/pkg/predicate"
	"github.com/mariadb-operator/mariadb-operator/pkg/watch"
	corev1 "k8s.io/api/core/v1"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	maxScaleMetricsPasswordSecretFieldPath = ".spec.auth.metricsPasswordSecretKeyRef.name"

	maxscaleTLSAdminCASecretFieldPath      = ".spec.tls.adminCASecretRef"
	maxscaleTLSAdminCertSecretFieldPath    = ".spec.tls.adminCertSecretRef"
	maxscaleTLSListenerCASecretFieldPath   = ".spec.tls.listenerCASecretRef"
	maxscaleTLSListenerCertSecretFieldPath = ".spec.tls.listenerCertSecretRef"
	maxscaleTLSServerCASecretFieldPath     = ".spec.tls.serverCASecretRef"
	maxscaleTLSServerCertSecretFieldPath   = ".spec.tls.serverCertSecretRef"
)

// nolint:gocyclo
// IndexerFuncForFieldPath returns an indexer function for a given field path.
func (m *MaxScale) IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error) {
	switch fieldPath {
	case maxScaleMetricsPasswordSecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.AreMetricsEnabled() && maxscale.Spec.Auth.MetricsPasswordSecretKeyRef.Name != "" {
				return []string{maxscale.Spec.Auth.MetricsPasswordSecretKeyRef.Name}
			}
			return nil
		}, nil
	case maxscaleTLSAdminCASecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.IsTLSEnabled() && maxscale.Spec.TLS != nil && maxscale.Spec.TLS.AdminCASecretRef != nil {
				return []string{maxscale.Spec.TLS.AdminCASecretRef.Name}
			}
			return nil
		}, nil
	case maxscaleTLSAdminCertSecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.IsTLSEnabled() && maxscale.Spec.TLS != nil && maxscale.Spec.TLS.AdminCertSecretRef != nil {
				return []string{maxscale.Spec.TLS.AdminCertSecretRef.Name}
			}
			return nil
		}, nil
	case maxscaleTLSListenerCASecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.IsTLSEnabled() && maxscale.Spec.TLS != nil && maxscale.Spec.TLS.ListenerCASecretRef != nil {
				return []string{maxscale.Spec.TLS.ListenerCASecretRef.Name}
			}
			return nil
		}, nil
	case maxscaleTLSListenerCertSecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.IsTLSEnabled() && maxscale.Spec.TLS != nil && maxscale.Spec.TLS.ListenerCertSecretRef != nil {
				return []string{maxscale.Spec.TLS.ListenerCertSecretRef.Name}
			}
			return nil
		}, nil
	case maxscaleTLSServerCASecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.IsTLSEnabled() && maxscale.Spec.TLS != nil && maxscale.Spec.TLS.ServerCASecretRef != nil {
				return []string{maxscale.Spec.TLS.ServerCASecretRef.Name}
			}
			return nil
		}, nil
	case maxscaleTLSServerCertSecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.IsTLSEnabled() && maxscale.Spec.TLS != nil && maxscale.Spec.TLS.ServerCertSecretRef != nil {
				return []string{maxscale.Spec.TLS.ServerCertSecretRef.Name}
			}
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unsupported field path: %s", fieldPath)
	}
}

// IndexMaxScale watches and indexes external resources referred by MaxScale resources.
func IndexMaxScale(ctx context.Context, mgr manager.Manager, builder *ctrlbuilder.Builder, client client.Client) error {
	watcherIndexer := watch.NewWatcherIndexer(mgr, builder, client)

	secretFieldPaths := []string{
		maxScaleMetricsPasswordSecretFieldPath,
		maxscaleTLSAdminCASecretFieldPath,
		maxscaleTLSAdminCertSecretFieldPath,
		maxscaleTLSListenerCASecretFieldPath,
		maxscaleTLSListenerCertSecretFieldPath,
		maxscaleTLSServerCASecretFieldPath,
		maxscaleTLSServerCertSecretFieldPath,
	}
	for _, fieldPath := range secretFieldPaths {
		if err := watcherIndexer.Watch(
			ctx,
			&corev1.Secret{},
			&MaxScale{},
			&MaxScaleList{},
			fieldPath,
			ctrlbuilder.WithPredicates(
				predicate.PredicateWithLabel(metadata.WatchLabel),
			),
		); err != nil {
			return fmt.Errorf("error watching '%s': %v", fieldPath, err)
		}
	}

	return nil
}
