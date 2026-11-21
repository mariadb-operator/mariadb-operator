package galera

import (
	"context"
	"fmt"
	"time"
)

type podSyncClock interface {
	Now() time.Time
	After(time.Duration) <-chan time.Time
}

type realPodSyncClock struct{}

func (realPodSyncClock) Now() time.Time                         { return time.Now() }
func (realPodSyncClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

type recoverySyncState struct {
	cluster string
	local   string
}

func (s recoverySyncState) ready(bootstrap bool) bool {
	return s.cluster == "Primary" && (s.local == "Synced" || (bootstrap && s.local == "Donor/Desynced"))
}

// A joiner may not accept SQL connections while mariadb-backup transfers its
// state. Wait for the full PodSyncTimeout without scheduling a Pod deletion.
// Recreating an explicitly stopped container is handled separately by the
// caller, before starting this wait.
func waitForRecoveryJoiner(ctx context.Context, clock podSyncClock, deadline time.Time,
	observe func(context.Context) (recoverySyncState, error)) error {
	var state recoverySyncState
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		state, lastErr = observe(ctx)
		if lastErr == nil && state.ready(false) {
			return nil
		}
		remaining := deadline.Sub(clock.Now())
		if remaining <= 0 {
			return fmt.Errorf("pod synchronization timed out (cluster=%s, state=%s, last-error=%v): %w",
				state.cluster, state.local, lastErr, context.DeadlineExceeded)
		}
		interval := time.Second
		if remaining < interval {
			interval = remaining
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-clock.After(interval):
		}
	}
}
