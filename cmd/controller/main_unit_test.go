package main

import (
	"context"
	"testing"
	"time"

	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

func TestLeaderElectionFlagDefaults(t *testing.T) {
	tests := []struct {
		name string
		flag string
		want time.Duration
	}{
		{
			name: "lease duration defaults to the controller-runtime value",
			flag: "leader-elect-lease-duration",
			want: 15 * time.Second,
		},
		{
			name: "renew deadline defaults to the controller-runtime value",
			flag: "leader-elect-renew-deadline",
			want: 10 * time.Second,
		},
		{
			name: "retry period defaults to the controller-runtime value",
			flag: "leader-elect-retry-period",
			want: 2 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := rootCmd.PersistentFlags().Lookup(tt.flag)
			if flag == nil {
				t.Fatalf("expected flag %q to be registered", tt.flag)
			}
			got, err := time.ParseDuration(flag.DefValue)
			if err != nil {
				t.Fatalf("unexpected error parsing default of %q: %v", tt.flag, err)
			}
			if got != tt.want {
				t.Errorf("expected default of %q to be %v, got %v", tt.flag, tt.want, got)
			}
		})
	}
}

func TestLeaderElectionDefaultsAreValid(t *testing.T) {
	tests := []struct {
		name          string
		leaseDuration time.Duration
		renewDeadline time.Duration
		retryPeriod   time.Duration
		wantErr       bool
	}{
		{
			name:          "defaults are accepted by client-go",
			leaseDuration: defaultLeaderElectLeaseDuration,
			renewDeadline: defaultLeaderElectRenewDeadline,
			retryPeriod:   defaultLeaderElectRetryPeriod,
			wantErr:       false,
		},
		{
			name:          "headroom values from a slow API server are accepted",
			leaseDuration: 60 * time.Second,
			renewDeadline: 40 * time.Second,
			retryPeriod:   5 * time.Second,
			wantErr:       false,
		},
		{
			name:          "renew deadline above lease duration is rejected",
			leaseDuration: defaultLeaderElectRenewDeadline,
			renewDeadline: defaultLeaderElectLeaseDuration,
			retryPeriod:   defaultLeaderElectRetryPeriod,
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
				Lock: &resourcelock.LeaseLock{
					LockConfig: resourcelock.ResourceLockConfig{
						Identity: "mariadb-operator-0",
					},
				},
				LeaseDuration: tt.leaseDuration,
				RenewDeadline: tt.renewDeadline,
				RetryPeriod:   tt.retryPeriod,
				Callbacks: leaderelection.LeaderCallbacks{
					OnStartedLeading: func(_ context.Context) {},
					OnStoppedLeading: func() {},
				},
			})
			if tt.wantErr && err == nil {
				t.Error("expected an error, got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
