package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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

func (b *PhysicalBackup) ServiceAccountKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      ptr.Deref(b.Spec.ServiceAccountName, b.Name),
		Namespace: b.Namespace,
	}
}

func (b *PhysicalBackup) RoleKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-role", b.ServiceAccountKey().Name),
		Namespace: b.Namespace,
	}
}

func (b *PhysicalBackup) RoleBindingKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-rolebinding", b.ServiceAccountKey().Name),
		Namespace: b.Namespace,
	}
}
