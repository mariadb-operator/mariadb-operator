package galera

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/mariadb-operator/agent/pkg/client"
	"github.com/mariadb-operator/agent/pkg/galera"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// TODO: perform galera recovery orchestrating requests to agents
// See:
// https://github.com/mariadb-operator/mariadb-ha-poc/blob/main/galera/kubernetes/2-crashrecovery.cnf
// https://github.com/mariadb-operator/mariadb-ha-poc/blob/main/galera/kubernetes/1-bootstrap.cnf
//	Maybe useful for recovery?
//	- SHOW STATUS LIKE 'wsrep_local_state_comment';
//		The output will display the state of each node, such as "Synced," "Donor," "Joining," "Joined," or "Disconnected."
//		All nodes should ideally be in the "Synced" state.

type recoveryStatus struct {
	stateByPod     map[int]*galera.GaleraState
	bootstrapByPod map[int]*galera.Bootstrap
	mux            *sync.RWMutex
}

func newRecoveryStatus() *recoveryStatus {
	return &recoveryStatus{
		stateByPod:     make(map[int]*galera.GaleraState),
		bootstrapByPod: make(map[int]*galera.Bootstrap),
		mux:            &sync.RWMutex{},
	}
}

type bootstrapSource struct {
	bootstrap *galera.Bootstrap
	podIndex  int
}

func (status *recoveryStatus) safeToBootstrap() *bootstrapSource {
	status.mux.RLock()
	defer status.mux.RUnlock()
	for k, v := range status.stateByPod {
		if v.SafeToBootstrap {
			return &bootstrapSource{
				bootstrap: &galera.Bootstrap{
					UUID:  v.UUID,
					Seqno: v.Seqno,
				},
				podIndex: k,
			}
		}
	}
	return nil
}

func (r *GaleraReconciler) reconcileGaleraRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) error {
	logger := log.FromContext(ctx).WithName("galera-recovery")
	logger.Info("Recovering cluster")

	sts, err := r.statefulSet(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error getting StatefulSet: %v", err)
	}

	// TODO: handle cases where only a few Pods are not Ready.
	// Check status of each node with:
	//	- SHOW STATUS LIKE 'wsrep_local_state_comment';
	//		The output will display the state of each node, such as "Synced," "Donor," "Joining," "Joined," or "Disconnected."
	//		All nodes should ideally be in the "Synced" state.
	if sts.Status.ReadyReplicas != 0 {
		return nil
	}

	clientSet, err := newAgentClientSet(mariadb, client.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("error getting agent client: %v", err)
	}

	pods, err := r.pods(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error getting Pods: %v", err)
	}
	logger.V(1).Info("Pods", "pods", len(pods))

	status := newRecoveryStatus()

	if err := r.stateByPod(ctx, pods, status, clientSet, logger); err != nil {
		return fmt.Errorf("error getting Galera state by Pod: %v", err)
	}
	logger.V(1).Info("State by Pod", "state", status.stateByPod)

	logger.V(1).Info("Checking SafeToBootstrap")
	if source := status.safeToBootstrap(); source != nil {
		logger.Info("Bootstrapping cluster", "pod-index", source.podIndex)
		if err := r.bootstrap(ctx, source, clientSet, logger); err != nil {
			return fmt.Errorf("error bootstrapping from Pod index: %d", source.podIndex)
		}
		return nil
	}

	if err := r.recoveryByPod(ctx, mariadb, pods, status, clientSet, logger); err != nil {
		return fmt.Errorf("error getting bootstrap by Pod: %v", err)
	}
	logger.V(1).Info("Bootstrap by Pod", "bootstrap", status.bootstrapByPod)

	return nil
}

func (r *GaleraReconciler) pods(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) ([]corev1.Pod, error) {
	var pods []corev1.Pod
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		key := types.NamespacedName{
			Name:      statefulset.PodName(mariadb.ObjectMeta, i),
			Namespace: mariadb.Namespace,
		}
		var pod corev1.Pod
		err := r.Get(ctx, key, &pod)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("error getting Pod '%s': %v", key.Name, err)
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

func (r *GaleraReconciler) stateByPod(ctx context.Context, pods []corev1.Pod, status *recoveryStatus,
	clientSet *agentClientSet, logger logr.Logger) error {
	doneChan := make(chan struct{})
	errChan := make(chan error)

	var wg sync.WaitGroup
	for _, pod := range pods {
		i, err := statefulset.PodIndex(pod.Name)
		if err != nil {
			return fmt.Errorf("error getting index for Pod '%s': %v", pod.Name, err)
		}

		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			client, err := clientSet.clientForIndex(i)
			if err != nil {
				errChan <- fmt.Errorf("error getting client for Pod index '%d': %v", i, err)
				return
			}

			stateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err = r.pollWithTimeout(stateCtx, logger, func(ctx context.Context) error {
				galeraState, err := client.GaleraState.Get(ctx)
				if err != nil {
					return err
				}
				status.mux.Lock()
				status.stateByPod[i] = galeraState
				status.mux.Unlock()
				return nil
			}); err != nil {
				errChan <- fmt.Errorf("error getting Galera state for Pod index '%d': %v", i, err)
			}
		}(*i)
	}
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneChan:
		return nil
	case err := <-errChan:
		return err
	}
}

func (r *GaleraReconciler) recoveryByPod(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, pods []corev1.Pod, status *recoveryStatus,
	clientSet *agentClientSet, logger logr.Logger) error {
	doneChan := make(chan struct{})
	errChan := make(chan error)

	var wg sync.WaitGroup
	for _, pod := range pods {
		i, err := statefulset.PodIndex(pod.Name)
		if err != nil {
			return fmt.Errorf("error getting index for Pod '%s': %v", pod.Name, err)
		}

		wg.Add(1)
		go func(i int, pod *corev1.Pod) {
			defer wg.Done()

			client, err := clientSet.clientForIndex(i)
			if err != nil {
				errChan <- fmt.Errorf("error getting client for Pod index '%d': %v", i, err)
				return
			}

			logger.V(1).Info("enabling recovery", "pod-index", i)
			recoveryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err = r.pollWithTimeout(recoveryCtx, logger, func(ctx context.Context) error {
				return client.Recovery.Enable(ctx)
			}); err != nil {
				errChan <- fmt.Errorf("error enabling recovery in Pod index '%d': %v", i, err)
				return
			}

			deleteCtx, cancelDelete := context.WithCancel(ctx)
			defer cancelDelete()
			go func() {
				ticker := time.NewTicker(30 * time.Second)
				for {
					select {
					case <-deleteCtx.Done():
						logger.V(1).Info("stopping Pod deletion", "pod-index", i)
						return
					case <-ticker.C:
						logger.V(1).Info("deleting Pod", "pod-index", i)
						if err := r.Delete(ctx, pod); err != nil {
							logger.V(1).Error(err, "error deleting Pod", "pod-index", i)
						}
					}
				}
			}()

			logger.V(1).Info("performing recovery", "pod-index", i)
			recoveryCtx, cancel = context.WithTimeout(ctx, mariadb.Spec.Galera.Recovery.TimeoutOrDefault())
			defer cancel()
			if err = r.pollWithTimeout(recoveryCtx, logger, func(ctx context.Context) error {
				bootstrap, err := client.Recovery.Start(ctx)
				if err != nil {
					return err
				}
				status.mux.Lock()
				status.bootstrapByPod[i] = bootstrap
				status.mux.Unlock()
				return nil
			}); err != nil {
				errChan <- fmt.Errorf("error performing recovery in Pod index '%d': %v", i, err)
				return
			}
			cancelDelete()

			logger.V(1).Info("disabling recovery", "pod-index", i)
			recoveryCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err = r.pollWithTimeout(recoveryCtx, logger, func(ctx context.Context) error {
				return client.Recovery.Disable(ctx)
			}); err != nil {
				errChan <- fmt.Errorf("error disabling recovery in Pod index '%d': %v", i, err)
			}
		}(*i, &pod)
	}
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneChan:
		return nil
	case err := <-errChan:
		return err
	}
}

func (r *GaleraReconciler) bootstrap(ctx context.Context, source *bootstrapSource, clientSet *agentClientSet, logger logr.Logger) error {
	client, err := clientSet.clientForIndex(source.podIndex)
	if err != nil {
		return fmt.Errorf("error getting client for Pod index '%d': %v", source.podIndex, err)
	}

	bootstrapCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err = r.pollWithTimeout(bootstrapCtx, logger, func(ctx context.Context) error {
		return client.Bootstrap.Enable(ctx, source.bootstrap)
	}); err != nil {
		return fmt.Errorf("error enabling bootstrap in Pod index '%d': %v", source.podIndex, err)
	}
	return nil
}

func (r *GaleraReconciler) pollWithTimeout(ctx context.Context, logger logr.Logger, fn func(ctx context.Context) error) error {
	// TODO: bump apimachinery and migrate to PollUntilContextTimeout.
	// See: https://pkg.go.dev/k8s.io/apimachinery@v0.27.2/pkg/util/wait#PollUntilContextTimeout
	if err := wait.PollImmediateUntilWithContext(ctx, 1*time.Second, func(ctx context.Context) (bool, error) {
		err := fn(ctx)
		if err != nil {
			logger.V(1).Error(err, "error polling")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}
