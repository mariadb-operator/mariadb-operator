package wait

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func PollUntilSucessOrContextCancel(ctx context.Context, logger logr.Logger, fn func(ctx context.Context) error) error {
	return kwait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		if err := fn(ctx); err != nil {
			logger.V(1).Info("Error polling", "err", err)
			return false, nil
		}
		return true, nil
	})
}

func PollWithMariaDB(ctx context.Context, mariadbKey types.NamespacedName, client ctrlclient.Client, logger logr.Logger,
	fn func(ctx context.Context) error) error {
	return PollUntilSucessOrContextCancel(ctx, logger, func(ctx context.Context) error {
		if shouldPoll(ctx, mariadbKey, client, logger) {
			return fn(ctx)
		}
		return nil
	})
}

func shouldPoll(ctx context.Context, mariadbKey types.NamespacedName, client ctrlclient.Client, logger logr.Logger) bool {
	var mdb mariadbv1alpha1.MariaDB
	if err := client.Get(ctx, mariadbKey, &mdb); err != nil {
		logger.V(1).Info("Error getting MariaDB", "err", err)
		return !apierrors.IsNotFound(err)
	}
	return true
}
