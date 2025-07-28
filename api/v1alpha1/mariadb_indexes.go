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
	mariadbMyCnfConfigMapFieldPath = ".spec.myCnfConfigMapKeyRef.name"

	mariadbMetricsPasswordSecretFieldPath = ".spec.metrics.passwordSecretKeyRef"

	mariadbTLSServerCASecretFieldPath   = ".spec.tls.serverCASecretRef"
	mariadbTLSServerCertSecretFieldPath = ".spec.tls.serverCertSecretRef"
	mariadbTLSClientCASecretFieldPath   = ".spec.tls.clientCASecretRef"
	mariadbTLSClientCertSecretFieldPath = ".spec.tls.clientCertSecretRef"
)

// nolint:gocyclo
// IndexerFuncForFieldPath returns an indexer function for a given field path.
func (m *MariaDB) IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error) {
	switch fieldPath {
	case mariadbMyCnfConfigMapFieldPath:
		return func(obj client.Object) []string {
			mdb, ok := obj.(*MariaDB)
			if !ok {
				return nil
			}
			if mdb.Spec.MyCnfConfigMapKeyRef != nil && mdb.Spec.MyCnfConfigMapKeyRef.Name != "" {
				return []string{mdb.Spec.MyCnfConfigMapKeyRef.Name}
			}
			return nil
		}, nil
	case mariadbMetricsPasswordSecretFieldPath:
		return func(obj client.Object) []string {
			mdb, ok := obj.(*MariaDB)
			if !ok {
				return nil
			}
			if mdb.AreMetricsEnabled() && mdb.Spec.Metrics != nil && mdb.Spec.Metrics.PasswordSecretKeyRef.Name != "" {
				return []string{mdb.Spec.Metrics.PasswordSecretKeyRef.Name}
			}
			return nil
		}, nil
	case mariadbTLSServerCASecretFieldPath:
		return func(o client.Object) []string {
			mdb, ok := o.(*MariaDB)
			if !ok {
				return nil
			}
			if mdb.IsTLSEnabled() {
				return []string{mdb.TLSServerCASecretKey().Name}
			}
			return nil
		}, nil
	case mariadbTLSServerCertSecretFieldPath:
		return func(o client.Object) []string {
			mdb, ok := o.(*MariaDB)
			if !ok {
				return nil
			}
			if mdb.IsTLSEnabled() {
				return []string{mdb.TLSServerCertSecretKey().Name}
			}
			return nil
		}, nil
	case mariadbTLSClientCASecretFieldPath:
		return func(o client.Object) []string {
			mdb, ok := o.(*MariaDB)
			if !ok {
				return nil
			}
			if mdb.IsTLSEnabled() {
				return []string{mdb.TLSClientCASecretKey().Name}
			}
			return nil
		}, nil
	case mariadbTLSClientCertSecretFieldPath:
		return func(o client.Object) []string {
			mdb, ok := o.(*MariaDB)
			if !ok {
				return nil
			}
			if mdb.IsTLSEnabled() {
				return []string{mdb.TLSClientCertSecretKey().Name}
			}
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unsupported field path: %s", fieldPath)
	}
}

// IndexMariaDB watches and indexes external resources referred by MariaDB resources.
func IndexMariaDB(ctx context.Context, mgr manager.Manager, builder *ctrlbuilder.Builder, client client.Client) error {
	watcherIndexer := watch.NewWatcherIndexer(mgr, builder, client)

	if err := watcherIndexer.Watch(
		ctx,
		&corev1.ConfigMap{},
		&MariaDB{},
		&MariaDBList{},
		mariadbMyCnfConfigMapFieldPath,
		ctrlbuilder.WithPredicates(
			predicate.PredicateWithLabel(metadata.WatchLabel),
		),
	); err != nil {
		return fmt.Errorf("error watching '%s': %v", mariadbMyCnfConfigMapFieldPath, err)
	}

	secretFieldPaths := []string{
		mariadbMetricsPasswordSecretFieldPath,
		mariadbTLSServerCASecretFieldPath,
		mariadbTLSServerCertSecretFieldPath,
		mariadbTLSClientCASecretFieldPath,
		mariadbTLSClientCertSecretFieldPath,
	}
	for _, fieldPath := range secretFieldPaths {
		if err := watcherIndexer.Watch(
			ctx,
			&corev1.Secret{},
			&MariaDB{},
			&MariaDBList{},
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
