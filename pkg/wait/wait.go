package wait

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

func PollUntilSucessWithTimeout(ctx context.Context, logger logr.Logger, fn func(ctx context.Context) error) error {
	if err := kwait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		if err := fn(ctx); err != nil {
			logger.V(1).Info("Error polling", "err", err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}
