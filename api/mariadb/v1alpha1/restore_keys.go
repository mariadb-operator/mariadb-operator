package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

func (r *Restore) StagingPVCKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-staging", r.Name),
		Namespace: r.Namespace,
	}
}
