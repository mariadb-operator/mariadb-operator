package conditions

import (
	"context"
	"fmt"
	"reflect"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Patcher func(Conditioner)

type Ready struct{}

func NewReady() *Ready {
	return &Ready{}
}

func (p *Ready) FailedPatcher(msg string) Patcher {
	return func(c Conditioner) {
		SetReadyFailedWithMessage(c, msg)
	}
}

func (p *Ready) PatcherWithError(err error) Patcher {
	return func(c Conditioner) {
		if err == nil {
			SetReadyCreated(c)
		} else {
			SetReadyFailed(c)
		}
	}
}

func (p *Ready) RefResolverPatcher(err error, obj interface{}) Patcher {
	return func(c Conditioner) {
		if err == nil {
			return
		}
		if apierrors.IsNotFound(err) {
			SetReadyFailedWithMessage(c, fmt.Sprintf("%s not found", getType(obj)))
			return
		}
		SetReadyFailedWithMessage(c, fmt.Sprintf("Error getting %s", getType(obj)))
	}
}

type Complete struct {
	client client.Client
}

func NewComplete(client client.Client) *Complete {
	return &Complete{
		client: client,
	}
}

func (p *Complete) FailedPatcher(msg string) Patcher {
	return func(c Conditioner) {
		SetCompleteFailedWithMessage(c, msg)
	}
}

func (p *Complete) PatcherWithCronJob(ctx context.Context, err error, key types.NamespacedName) (Patcher, error) {
	if err != nil {
		return func(c Conditioner) {
			SetCompleteFailedWithMessage(c, "Error creating CronJob")
		}, nil
	}

	var cronJob batchv1.CronJob
	if err := p.client.Get(ctx, key, &cronJob); err != nil {
		return nil, err
	}
	return func(c Conditioner) {
		SetCompleteWithCronJob(c, &cronJob)
	}, nil
}

func (p *Complete) PatcherWithJob(ctx context.Context, err error, key types.NamespacedName) (Patcher, error) {
	if err != nil {
		return func(c Conditioner) {
			SetCompleteFailedWithMessage(c, "Error creating Job")
		}, nil
	}

	var job batchv1.Job
	if err := p.client.Get(ctx, key, &job); err != nil {
		return nil, err
	}
	return func(c Conditioner) {
		SetCompleteWithJob(c, &job)
	}, nil
}

func (p *Complete) RefResolverPatcher(err error, obj runtime.Object) Patcher {
	return func(c Conditioner) {
		if err == nil {
			return
		}
		if apierrors.IsNotFound(err) {
			SetCompleteFailedWithMessage(c, fmt.Sprintf("%s not found", getType(obj)))
			return
		}
		SetCompleteFailedWithMessage(c, fmt.Sprintf("Error getting %s", getType(obj)))
	}
}

func getType(obj interface{}) string {
	if t := reflect.TypeOf(obj); t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	} else {
		return t.Name()
	}
}
