package galera

import (
	"context"
	"errors"
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
	GaleraRecovery *mariadbv1alpha1.GaleraRecoveryStatus
	mux            *sync.RWMutex
}

func newRecoveryStatus(mariadb *mariadbv1alpha1.MariaDB) *recoveryStatus {
	var status mariadbv1alpha1.GaleraRecoveryStatus
	if mariadb.Status.GaleraRecovery != nil {
		status = *mariadb.Status.GaleraRecovery
	} else {
		status = mariadbv1alpha1.GaleraRecoveryStatus{
			State:     make(map[string]*galera.GaleraState),
			Recovered: make(map[string]*galera.Bootstrap),
		}
	}
	return &recoveryStatus{
		GaleraRecovery: &status,
		mux:            &sync.RWMutex{},
	}
}

type bootstrapSource struct {
	bootstrap *galera.Bootstrap
	pod       string
}

func (rs *recoveryStatus) safeToBootstrap() *bootstrapSource {
	rs.mux.RLock()
	defer rs.mux.RUnlock()
	for k, v := range rs.GaleraRecovery.State {
		if v.SafeToBootstrap && v.Seqno != -1 {
			return &bootstrapSource{
				bootstrap: &galera.Bootstrap{
					UUID:  v.UUID,
					Seqno: v.Seqno,
				},
				pod: k,
			}
		}
	}
	return nil
}

func (rs *recoveryStatus) isComplete(pods []corev1.Pod) bool {
	rs.mux.RLock()
	defer rs.mux.RUnlock()
	for _, p := range pods {
		if rs.GaleraRecovery.State[p.Name] == nil || rs.GaleraRecovery.Recovered[p.Name] == nil {
			return false
		}
	}
	return true
}

func (rs *recoveryStatus) bootstrapWithHighestSeqno(pods []corev1.Pod) (*bootstrapSource, error) {
	if len(pods) == 0 {
		return nil, errors.New("no Pods provided")
	}
	if source := rs.safeToBootstrap(); source != nil {
		return source, nil
	}

	rs.mux.RLock()
	defer rs.mux.RUnlock()
	var currentSoure galera.GaleraRecoverer
	var currentPod string

	for _, p := range pods {
		state := rs.GaleraRecovery.State[p.Name]
		recovered := rs.GaleraRecovery.Recovered[p.Name]
		if state != nil && state.GetSeqno() != -1 && state.Compare(currentSoure) >= 0 {
			currentSoure = state
			currentPod = p.Name
		}
		if recovered != nil && recovered.GetSeqno() != -1 && recovered.Compare(currentSoure) >= 0 {
			currentSoure = state
			currentPod = p.Name
		}
	}
	if currentSoure == nil {
		return nil, errors.New("bootstrap source not found")
	}
	return &bootstrapSource{
		bootstrap: &galera.Bootstrap{
			UUID:  currentSoure.GetUUID(),
			Seqno: currentSoure.GetSeqno(),
		},
		pod: currentPod,
	}, nil
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

	recoveryStatus := newRecoveryStatus(mariadb)

	if err := r.stateByPod(ctx, pods, recoveryStatus, clientSet, logger); err != nil {
		return fmt.Errorf("error getting Galera state by Pod: %v", err)
	}
	logger.V(1).Info("State by Pod", "state", recoveryStatus.GaleraRecovery.State)

	if err := r.patchStatus(ctx, mariadb, func(mdbStatus *mariadbv1alpha1.MariaDBStatus) {
		mdbStatus.GaleraRecovery = recoveryStatus.GaleraRecovery
	}); err != nil {
		return fmt.Errorf("error updating MariaDB status: %v", err)
	}

	if recoveryStatus.isComplete(pods) {
		source, err := recoveryStatus.bootstrapWithHighestSeqno(pods)
		if err != nil {
			return fmt.Errorf("error getting bootstrap source: %v", err)
		}
		logger.Info("Bootstrapping cluster", "pod", source.pod)
		if err := r.bootstrap(ctx, source, clientSet, logger); err != nil {
			return fmt.Errorf("error bootstrapping from Pod: %s", source.pod)
		}
	}

	if err := r.recoveryByPod(ctx, mariadb, pods, recoveryStatus, clientSet, logger); err != nil {
		return fmt.Errorf("error getting bootstrap by Pod: %v", err)
	}
	logger.V(1).Info("Recovery by Pod", "bootstrap", recoveryStatus.GaleraRecovery.Recovered)

	if err := r.patchStatus(ctx, mariadb, func(mdbStatus *mariadbv1alpha1.MariaDBStatus) {
		mdbStatus.GaleraRecovery = recoveryStatus.GaleraRecovery
	}); err != nil {
		return fmt.Errorf("error updating MariaDB status: %v", err)
	}

	if !recoveryStatus.isComplete(pods) {
		return fmt.Errorf("recovery status not complete")
	}

	source, err := recoveryStatus.bootstrapWithHighestSeqno(pods)
	if err != nil {
		return fmt.Errorf("error getting bootstrap source: %v", err)
	}
	logger.Info("Bootstrapping cluster", "pod", source.pod)
	if err := r.bootstrap(ctx, source, clientSet, logger); err != nil {
		return fmt.Errorf("error bootstrapping from Pod: %s", source.pod)
	}

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
		status.mux.RLock()
		if _, ok := status.GaleraRecovery.State[pod.Name]; ok {
			logger.V(1).Info("skipping Pod state", "pod", pod.Name)
			continue
		}
		status.mux.RUnlock()

		i, err := statefulset.PodIndex(pod.Name)
		if err != nil {
			return fmt.Errorf("error getting index for Pod '%s': %v", pod.Name, err)
		}

		wg.Add(1)
		go func(i int, pod *corev1.Pod) {
			defer wg.Done()

			client, err := clientSet.clientForIndex(i)
			if err != nil {
				errChan <- fmt.Errorf("error getting client for Pod '%s': %v", pod.Name, err)
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
				status.GaleraRecovery.State[pod.Name] = galeraState
				status.mux.Unlock()
				return nil
			}); err != nil {
				errChan <- fmt.Errorf("error getting Galera state for Pod '%s': %v", pod.Name, err)
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

func (r *GaleraReconciler) recoveryByPod(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, pods []corev1.Pod, status *recoveryStatus,
	clientSet *agentClientSet, logger logr.Logger) error {
	doneChan := make(chan struct{})
	errChan := make(chan error)

	var wg sync.WaitGroup
	for _, pod := range pods {
		status.mux.RLock()
		if _, ok := status.GaleraRecovery.Recovered[pod.Name]; ok {
			logger.V(1).Info("skipping Pod recovery", "pod", pod.Name)
			continue
		}
		status.mux.RUnlock()

		i, err := statefulset.PodIndex(pod.Name)
		if err != nil {
			return fmt.Errorf("error getting index for Pod '%s': %v", pod.Name, err)
		}

		wg.Add(1)
		go func(i int, pod *corev1.Pod) {
			defer wg.Done()

			client, err := clientSet.clientForIndex(i)
			if err != nil {
				errChan <- fmt.Errorf("error getting client for Pod '%s': %v", pod.Name, err)
				return
			}

			logger.V(1).Info("enabling recovery", "pod", pod.Name)
			recoveryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err = r.pollWithTimeout(recoveryCtx, logger, func(ctx context.Context) error {
				return client.Recovery.Enable(ctx)
			}); err != nil {
				errChan <- fmt.Errorf("error enabling recovery in Pod '%s': %v", pod.Name, err)
				return
			}

			deleteCtx, cancelDelete := context.WithCancel(ctx)
			defer cancelDelete()
			go func() {
				ticker := time.NewTicker(30 * time.Second)
				for {
					select {
					case <-deleteCtx.Done():
						logger.V(1).Info("stopping Pod deletion", "pod", pod.Name)
						return
					case <-ticker.C:
						logger.V(1).Info("deleting Pod", "pod", pod.Name)
						if err := r.Delete(ctx, pod); err != nil {
							logger.V(1).Error(err, "error deleting Pod", "pod", pod.Name)
						}
					}
				}
			}()

			logger.V(1).Info("performing recovery", "pod", pod.Name)
			recoveryCtx, cancel = context.WithTimeout(ctx, mariadb.Spec.Galera.Recovery.TimeoutOrDefault())
			defer cancel()
			if err = r.pollWithTimeout(recoveryCtx, logger, func(ctx context.Context) error {
				bootstrap, err := client.Recovery.Start(ctx)
				if err != nil {
					return err
				}
				status.mux.Lock()
				status.GaleraRecovery.Recovered[pod.Name] = bootstrap
				status.mux.Unlock()
				return nil
			}); err != nil {
				errChan <- fmt.Errorf("error performing recovery in Pod '%s': %v", pod.Name, err)
				return
			}
			cancelDelete()

			logger.V(1).Info("disabling recovery", "pod", pod.Name)
			recoveryCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err = r.pollWithTimeout(recoveryCtx, logger, func(ctx context.Context) error {
				return client.Recovery.Disable(ctx)
			}); err != nil {
				errChan <- fmt.Errorf("error disabling recovery in Pod '%s': %v", pod.Name, err)
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
	idx, err := statefulset.PodIndex(source.pod)
	if err != nil {
		return fmt.Errorf("error getting index for Pod '%s': %v", source.pod, err)
	}

	client, err := clientSet.clientForIndex(*idx)
	if err != nil {
		return fmt.Errorf("error getting client for Pod '%s': %v", source.pod, err)
	}

	bootstrapCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err = r.pollWithTimeout(bootstrapCtx, logger, func(ctx context.Context) error {
		return client.Bootstrap.Enable(ctx, source.bootstrap)
	}); err != nil {
		return fmt.Errorf("error enabling bootstrap in Pod '%s': %v", source.pod, err)
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
