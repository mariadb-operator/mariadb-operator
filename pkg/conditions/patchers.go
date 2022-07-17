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
			SetReadyCreated(c)
		} else {
			SetReadyFailed(c)
		}
	}
}

func NewConditionReadyFailedPatcher(msg string) ConditionPatcher {
	return func(c Conditioner) {
		SetReadyFailedWithMessage(c, msg)
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

func (p *ConditionComplete) FailedPatcher(msg string) ConditionPatcher {
	return func(c Conditioner) {
		SetCompleteFailedWithMessage(c, msg)
	}
}

func (p *ConditionComplete) PatcherWithJob(ctx context.Context, err error, jobKey types.NamespacedName) (ConditionPatcher, error) {
	if err != nil {
		return func(c Conditioner) {
			SetCompleteFailedWithMessage(c, "Failed creating Job")
		}, nil
	}

	var job batchv1.Job
	if err := p.client.Get(ctx, jobKey, &job); err != nil {
		return nil, err
	}
	return func(c Conditioner) {
		SetCompleteWithJob(c, &job)
	}, nil
}
