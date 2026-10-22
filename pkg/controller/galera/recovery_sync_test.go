package galera

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/galera/recovery"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type fakePodSyncClock struct{ now time.Time }

func (c *fakePodSyncClock) Now() time.Time { return c.now }
func (c *fakePodSyncClock) After(d time.Duration) <-chan time.Time {
	c.now = c.now.Add(d)
	ch := make(chan time.Time, 1)
	ch <- c.now
	return ch
}

type deleteRecorder struct {
	ctrlclient.Client
	deletions    []ctrlclient.DeleteOptions
	beforeDelete func(context.Context, ctrlclient.Object) error
}

func (c *deleteRecorder) Delete(ctx context.Context, obj ctrlclient.Object, options ...ctrlclient.DeleteOption) error {
	var opts ctrlclient.DeleteOptions
	for _, option := range options {
		option.ApplyToDelete(&opts)
	}
	c.deletions = append(c.deletions, opts)
	if c.beforeDelete != nil {
		if err := c.beforeDelete(ctx, obj); err != nil {
			return err
		}
	}
	return c.Client.Delete(ctx, obj, options...)
}

func joinerFixture(t *testing.T, name string, state corev1.ContainerState) (*GaleraReconciler, *deleteRecorder, *corev1.Pod) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test", UID: "original-uid",
		ResourceVersion: "7"}, Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
		{Name: builder.MariadbContainerName, State: state},
	}}}
	client := &deleteRecorder{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()}
	return &GaleraReconciler{Client: client}, client, pod
}

func TestRecoveryJoinerPastTenSecondsCanBecomeDonorWithoutDeletion(t *testing.T) {
	for _, source := range []string{"mdb-0", "mdb-1", "mdb-2"} {
		t.Run(source, func(t *testing.T) {
			mdb := &mariadbv1alpha1.MariaDB{ObjectMeta: metav1.ObjectMeta{Name: "mdb"},
				Spec: mariadbv1alpha1.MariaDBSpec{Replicas: 3}}
			rs := newRecoveryStatus(mdb)
			for _, name := range []string{"mdb-0", "mdb-1", "mdb-2"} {
				seqno := 90
				if name == source {
					seqno = 100
				}
				rs.setRecovered(name, &recovery.Bootstrap{UUID: "8f59a04e-c307-11f1-9cec-82756d5a6cf6", Seqno: seqno})
			}
			selected, err := rs.bootstrapSource(mdb, nil, logr.Discard())
			if err != nil || selected.pod != source {
				t.Fatalf("highest recovered sequence was not selected: %v", err)
			}
			joiner := "mdb-0"
			if selected.pod == joiner {
				joiner = "mdb-1"
			}
			r, client, pod := joinerFixture(t, joiner, corev1.ContainerState{Running: &corev1.ContainerStateRunning{}})
			if err := r.restartFailedJoiner(context.Background(), ctrlclient.ObjectKeyFromObject(pod), logr.Discard()); err != nil {
				t.Fatal(err)
			}
			clock := &fakePodSyncClock{now: time.Unix(0, 0)}
			err = waitForRecoveryJoiner(context.Background(), clock, clock.Now().Add(time.Minute),
				func(context.Context) (recoverySyncState, error) {
					elapsed := clock.Now().Sub(time.Unix(0, 0))
					if elapsed < 15*time.Second {
						return recoverySyncState{}, errors.New("joiner SQL unavailable during mariadb-backup")
					}
					if elapsed < 45*time.Second {
						return recoverySyncState{cluster: "Primary", local: "Donor/Desynced"}, nil
					}
					return recoverySyncState{cluster: "Primary", local: "Synced"}, nil
				})
			if err != nil || len(client.deletions) != 0 || clock.Now().Sub(time.Unix(0, 0)) != 45*time.Second {
				t.Fatalf("ongoing SST was not preserved: %v, deletions=%d, time=%v", err, len(client.deletions), clock.Now())
			}
		})
	}
}

func TestRecoveryJoinerTimeoutPreservesRunningContainer(t *testing.T) {
	r, client, pod := joinerFixture(t, "mdb-0", corev1.ContainerState{Running: &corev1.ContainerStateRunning{}})
	if err := r.restartFailedJoiner(context.Background(), ctrlclient.ObjectKeyFromObject(pod), logr.Discard()); err != nil {
		t.Fatal(err)
	}
	clock := &fakePodSyncClock{now: time.Unix(0, 0)}
	err := waitForRecoveryJoiner(context.Background(), clock, clock.Now().Add(30*time.Second),
		func(context.Context) (recoverySyncState, error) {
			return recoverySyncState{cluster: "Primary", local: "Joining"}, nil
		})
	if !errors.Is(err, context.DeadlineExceeded) || len(client.deletions) != 0 ||
		clock.Now().Sub(time.Unix(0, 0)) != 30*time.Second {
		t.Fatalf("stuck joiner did not time out without deletion: %v", err)
	}
}

func TestRecoveryFailedJoinerCanBeRecreatedAndSynchronize(t *testing.T) {
	for _, state := range []corev1.ContainerState{
		{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}},
		{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
	} {
		r, client, pod := joinerFixture(t, "mdb-0", state)
		if err := r.restartFailedJoiner(context.Background(), ctrlclient.ObjectKeyFromObject(pod), logr.Discard()); err != nil {
			t.Fatal(err)
		}
		if len(client.deletions) != 1 || client.deletions[0].Preconditions == nil ||
			*client.deletions[0].Preconditions.UID != pod.UID || *client.deletions[0].Preconditions.ResourceVersion != "7" {
			t.Fatal("failed joiner was not recreated with UID and ResourceVersion preconditions")
		}
		replacement := pod.DeepCopy()
		replacement.UID = types.UID("replacement-uid")
		replacement.ResourceVersion = ""
		replacement.Status.ContainerStatuses[0].State = corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
		if err := client.Create(context.Background(), replacement); err != nil {
			t.Fatal(err)
		}
		if err := r.restartFailedJoiner(context.Background(), ctrlclient.ObjectKeyFromObject(replacement), logr.Discard()); err != nil {
			t.Fatal(err)
		}
		clock := &fakePodSyncClock{now: time.Unix(0, 0)}
		err := waitForRecoveryJoiner(context.Background(), clock, clock.Now().Add(30*time.Second),
			func(context.Context) (recoverySyncState, error) {
				if clock.Now().Sub(time.Unix(0, 0)) < 20*time.Second {
					return recoverySyncState{cluster: "Primary", local: "Joining"}, nil
				}
				return recoverySyncState{cluster: "Primary", local: "Synced"}, nil
			})
		if err != nil || len(client.deletions) != 1 {
			t.Fatalf("replacement failed to recover or was deleted again: %v", err)
		}
	}
}

func TestRecoveryPendingAndRunningContainersAreNotFailedJoiners(t *testing.T) {
	for _, state := range []corev1.ContainerState{
		{},
		{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
		{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}},
		{Running: &corev1.ContainerStateRunning{}},
	} {
		r, client, pod := joinerFixture(t, "mdb-0", state)
		if err := r.restartFailedJoiner(context.Background(), ctrlclient.ObjectKeyFromObject(pod), logr.Discard()); err != nil {
			t.Fatal(err)
		}
		if len(client.deletions) != 0 {
			t.Fatal("unconfirmed failure authorized deletion")
		}
	}
}

func TestRecoveryPrimaryDonorIsReadyOnlyAsBootstrapSource(t *testing.T) {
	state := recoverySyncState{cluster: "Primary", local: "Donor/Desynced"}
	if !state.ready(true) || state.ready(false) {
		t.Fatal("active source donor was rejected or ordinary donor was treated as synchronized")
	}
	if (recoverySyncState{cluster: "non-Primary", local: "Synced"}).ready(true) {
		t.Fatal("non-Primary source was accepted")
	}
}

func TestRecoveryJoinerRestartRejectsConcurrentRunningStatus(t *testing.T) {
	r, client, pod := joinerFixture(t, "mdb-0", corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}})
	client.beforeDelete = func(ctx context.Context, obj ctrlclient.Object) error {
		var current corev1.Pod
		if err := client.Get(ctx, ctrlclient.ObjectKeyFromObject(obj), &current); err != nil {
			return err
		}
		current.Status.ContainerStatuses[0].State = corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
		return client.Status().Update(ctx, &current)
	}
	err := r.restartFailedJoiner(context.Background(), ctrlclient.ObjectKeyFromObject(pod), logr.Discard())
	if !apierrors.IsConflict(err) {
		t.Fatalf("expected a precondition conflict after container became running, got %v", err)
	}
	var current corev1.Pod
	if err := client.Get(context.Background(), ctrlclient.ObjectKeyFromObject(pod), &current); err != nil {
		t.Fatalf("running replacement was deleted: %v", err)
	}
	if current.Status.ContainerStatuses[0].State.Running == nil {
		t.Fatal("running status was not preserved")
	}
}
