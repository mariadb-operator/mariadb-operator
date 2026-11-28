package pdb

import (
	"context"
	"fmt"
	"maps"

	policyv1 "k8s.io/api/policy/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PDBReconciler struct {
	client.Client
}

func NewPDBReconciler(client client.Client) *PDBReconciler {
	return &PDBReconciler{
		Client: client,
	}
}

// Reconcile creates the desired PodDisruptionBudget or, if it already exists, patches its availability settings.
// Desired labels and annotations are merged in, like the Service reconciler does, so metadata added by others is kept.
// The selector is left untouched.
func (r *PDBReconciler) Reconcile(ctx context.Context, desiredPDB *policyv1.PodDisruptionBudget) error {
	key := client.ObjectKeyFromObject(desiredPDB)
	var existingPDB policyv1.PodDisruptionBudget
	if err := r.Get(ctx, key, &existingPDB); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting PodDisruptionBudget: %v", err)
		}
		if err := r.Create(ctx, desiredPDB); err != nil {
			return fmt.Errorf("error creating PodDisruptionBudget: %v", err)
		}
		return nil
	}

	orig := existingPDB.DeepCopy()
	if existingPDB.Labels == nil {
		existingPDB.Labels = make(map[string]string)
	}
	if existingPDB.Annotations == nil {
		existingPDB.Annotations = make(map[string]string)
	}
	maps.Copy(existingPDB.Labels, desiredPDB.Labels)
	maps.Copy(existingPDB.Annotations, desiredPDB.Annotations)
	existingPDB.Spec.MinAvailable = desiredPDB.Spec.MinAvailable
	existingPDB.Spec.MaxUnavailable = desiredPDB.Spec.MaxUnavailable
	if apiequality.Semantic.DeepEqual(orig, &existingPDB) {
		return nil
	}
	patch := client.MergeFrom(orig)
	if err := r.Patch(ctx, &existingPDB, patch); err != nil {
		return fmt.Errorf("error patching PodDisruptionBudget: %v", err)
	}
	return nil
}

// DeleteControlled removes the PodDisruptionBudget when it exists and is controlled by the owner.
// A PodDisruptionBudget with the same name that the owner does not control is left alone.
func (r *PDBReconciler) DeleteControlled(ctx context.Context, key client.ObjectKey, owner metav1.Object) error {
	var existingPDB policyv1.PodDisruptionBudget
	if err := r.Get(ctx, key, &existingPDB); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error getting PodDisruptionBudget: %v", err)
	}
	if !metav1.IsControlledBy(&existingPDB, owner) {
		return nil
	}
	if err := r.Delete(ctx, &existingPDB); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error deleting PodDisruptionBudget: %v", err)
	}
	return nil
}
