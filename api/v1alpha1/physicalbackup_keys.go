package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

func (b *PhysicalBackup) StagingPVCKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-staging", b.Name),
		Namespace: b.Namespace,
	}
}

func (b *PhysicalBackup) StoragePVCKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      b.Name,
		Namespace: b.Namespace,
	}
}
