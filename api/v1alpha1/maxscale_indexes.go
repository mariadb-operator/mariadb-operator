package v1alpha1

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/predicate"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/watch"
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

	maxscaleMariaDbRefNameFieldPath = ".spec.mariaDbRef.name"
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
			if maxscale.IsTLSEnabled() {
				return []string{maxscale.TLSAdminCASecretKey().Name}
			}
			return nil
		}, nil
	case maxscaleTLSAdminCertSecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.IsTLSEnabled() {
				return []string{maxscale.TLSAdminCertSecretKey().Name}
			}
			return nil
		}, nil
	case maxscaleTLSListenerCASecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.IsTLSEnabled() {
				return []string{maxscale.TLSListenerCASecretKey().Name}
			}
			return nil
		}, nil
	case maxscaleTLSListenerCertSecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.IsTLSEnabled() {
				return []string{maxscale.TLSListenerCertSecretKey().Name}
			}
			return nil
		}, nil
	case maxscaleTLSServerCASecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.IsTLSEnabled() {
				return []string{maxscale.TLSServerCASecretKey().Name}
			}
			return nil
		}, nil
	case maxscaleTLSServerCertSecretFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.IsTLSEnabled() {
				return []string{maxscale.TLSServerCertSecretKey().Name}
			}
			return nil
		}, nil
	case maxscaleMariaDbRefNameFieldPath:
		return func(obj client.Object) []string {
			maxscale, ok := obj.(*MaxScale)
			if !ok {
				return nil
			}
			if maxscale.Spec.MariaDBRef != nil {
				return []string{maxscale.Spec.MariaDBRef.Name}
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

	if err := watcherIndexer.Watch(
		ctx,
		&MariaDB{},
		&MaxScale{},
		&MaxScaleList{},
		maxscaleMariaDbRefNameFieldPath,
	); err != nil {
		return fmt.Errorf("error watching '%s': %v", maxscaleMariaDbRefNameFieldPath, err)
	}

	return nil
}
