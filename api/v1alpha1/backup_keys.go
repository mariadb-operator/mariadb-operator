package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

func (b *Backup) StagingPVCKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-staging", b.Name),
		Namespace: b.Namespace,
	}
}

func (b *Backup) StoragePVCKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      b.Name,
		Namespace: b.Namespace,
	}
}
