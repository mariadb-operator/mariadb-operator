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
	MariaDB        *mariadbv1alpha1.MariaDB
	Key            types.NamespacedName
	MinAvailable   *intstr.IntOrString
	MaxUnavailable *intstr.IntOrString
	SelectorLabels map[string]string
}

func (b *Builder) BuildPodDisruptionBudget(opts *PodDisruptionBudgetOpts, owner metav1.Object) (*policyv1.PodDisruptionBudget, error) {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: pdbObjMeta(opts),
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

func pdbObjMeta(opts *PodDisruptionBudgetOpts) metav1.ObjectMeta {
	builder := metadata.NewMetadataBuilder(opts.Key)
	if opts.MariaDB != nil {
		builder = builder.WithMariaDB(opts.MariaDB)
	}
	return builder.Build()
}
