package v1alpha1

import (
	"context"
	"fmt"

	"github.com/mariadb-operator/mariadb-operator/pkg/watch"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	physicalBackupMetadataCtrlFieldPath = ".metadata.controller"
)

// nolint:gocyclo
// IndexerFuncForFieldPath returns an indexer function for a given field path.
func (m *PhysicalBackup) IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error) {
	switch fieldPath {
	case physicalBackupMetadataCtrlFieldPath:
		return func(o client.Object) []string {
			physicalBackup, ok := o.(*batchv1.Job)
			if !ok {
				return nil
			}

			owner := metav1.GetControllerOf(physicalBackup)
			if owner == nil {
				return nil
			}
			if owner.Kind != PhysicalBackupKind {
				return nil
			}
			if owner.APIVersion != GroupVersion.String() {
				return nil
			}

			return []string{owner.Name}
		}, nil
	default:
		return nil, fmt.Errorf("unsupported field path: %s", fieldPath)
	}
}

// IndexMaxScale watches and indexes external resources referred by PhysicalBackup resources.
func IndexPhysicalBackup(ctx context.Context, mgr manager.Manager, builder *ctrlbuilder.Builder, client client.Client) error {
	watcherIndexer := watch.NewWatcherIndexer(mgr, builder, client)

	if err := watcherIndexer.Watch(
		ctx,
		&batchv1.Job{},
		&PhysicalBackup{},
		&PhysicalBackupList{},
		physicalBackupMetadataCtrlFieldPath,
	); err != nil {
		return fmt.Errorf("error watching '%s': %v", physicalBackupMetadataCtrlFieldPath, err)
	}

	return nil
}
