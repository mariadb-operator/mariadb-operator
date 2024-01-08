package deployment

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeploymentReconciler struct {
	client.Client
}

func NewDeploymentReconciler(client client.Client) *DeploymentReconciler {
	return &DeploymentReconciler{
		Client: client,
	}
}

func (r *DeploymentReconciler) Reconcile(ctx context.Context, desiredDeploy *appsv1.Deployment) error {
	key := client.ObjectKeyFromObject(desiredDeploy)
	var existingDeploy appsv1.Deployment
	if err := r.Get(ctx, key, &existingDeploy); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting Deployment: %v", err)
		}
		if err := r.Create(ctx, desiredDeploy); err != nil {
			return fmt.Errorf("error creating Deployment: %v", err)
		}
		return nil
	}

	patch := client.MergeFrom(existingDeploy.DeepCopy())
	existingDeploy.Spec.Replicas = desiredDeploy.Spec.Replicas
	existingDeploy.Spec.Template = desiredDeploy.Spec.Template

	return r.Patch(ctx, &existingDeploy, patch)
}
