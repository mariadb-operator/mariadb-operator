package builder

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metadata "github.com/mariadb-operator/mariadb-operator/pkg/builder/metadata"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type PodDisruptionBudgetOpts struct {
	Metadata       *mariadbv1alpha1.Metadata
	Key            types.NamespacedName
	MinAvailable   *intstr.IntOrString
	MaxUnavailable *intstr.IntOrString
	SelectorLabels map[string]string
}

func (b *Builder) BuildPodDisruptionBudget(opts PodDisruptionBudgetOpts, owner metav1.Object) (*policyv1.PodDisruptionBudget, error) {
	objMeta :=
		metadata.NewMetadataBuilder(opts.Key).
			WithMetadata(opts.Metadata).
			Build()
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: objMeta,
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable:   opts.MinAvailable,
			MaxUnavailable: opts.MaxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: opts.SelectorLabels,
			},
		},
	}
	if err := controllerutil.SetControllerReference(owner, pdb, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to PodDisruptionBudget: %v", err)
	}
	return pdb, nil
}
