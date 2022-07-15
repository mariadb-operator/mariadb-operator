package conditions

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConditionPatcher func(Conditioner)

func NewConditionReadyPatcher(err error) ConditionPatcher {
	return func(c Conditioner) {
		if err == nil {
			SetConditionCreated(c)
		} else {
			SetConditionFailed(c)
		}
	}
}

type ConditionComplete struct {
	client client.Client
}

func NewConditionComplete(client client.Client) *ConditionComplete {
	return &ConditionComplete{
		client: client,
	}
}

func (p *ConditionComplete) Patcher(ctx context.Context, err error, jobKey types.NamespacedName) (ConditionPatcher, error) {
	if err != nil {
		return func(c Conditioner) {
			SetConditionFailedWithMessage(c, "Failed creating Job")
		}, nil
	}

	var job batchv1.Job
	if getErr := p.client.Get(ctx, jobKey, &job); getErr != nil {
		return nil, getErr
	}
	return func(c Conditioner) {
		SetConditionCompleteWithJob(c, &job)
	}, nil
}
