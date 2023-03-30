package builder

import (
	"fmt"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type PodDisruptionBudgetOpts struct {
	Key            types.NamespacedName
	MinAvailable   *intstr.IntOrString
	MaxUnavailable *intstr.IntOrString
	SelectorLabels map[string]string
}

func (b *Builder) BuildPodDisruptionBudget(opts *PodDisruptionBudgetOpts, owner metav1.Object) (*policyv1.PodDisruptionBudget, error) {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Key.Name,
			Namespace: opts.Key.Namespace,
		},
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
