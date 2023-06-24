package galera

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"github.com/mariadb-operator/agent/pkg/client"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	sqlclient "github.com/mariadb-operator/mariadb-operator/pkg/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *GaleraReconciler) reconcileRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, sts *appsv1.StatefulSet) error {
	pods, err := r.pods(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error getting Pods: %v", err)
	}
	agentClientSet, err := newAgentClientSet(mariadb, client.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("error getting agent client: %v", err)
	}
	sqlClientSet := sqlclient.NewClientSet(mariadb, r.RefResolver)
	defer sqlClientSet.Close()
	logger := log.FromContext(ctx).WithName("galera-recovery")

	if sts.Status.ReadyReplicas == 0 {
		return r.recoverCluster(ctx, mariadb, pods, agentClientSet, logger.WithName("cluster"))
	}
	return r.recoverPods(ctx, mariadb, pods, sqlClientSet, logger.WithName("pod"))
}

func (r *GaleraReconciler) recoverCluster(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, pods []corev1.Pod,
	clientSet *agentClientSet, logger logr.Logger) error {
	rs := newRecoveryStatus(mariadb)

	if rs.isBootstrapping() {
		// TODO: cluster recovery timeout at the top level?
		if rs.bootstrapTimeout(mariadb) {
			logger.Info("Bootstrap timed out. Resetting recovery status...")
			rs.reset()
			return r.patchRecoveryStatus(ctx, mariadb, rs)
		}
		return nil
	}

	logger.V(1).Info("State by Pod")
	var stateErr *multierror.Error
	err := r.stateByPod(ctx, pods, rs, clientSet, logger)
	stateErr = multierror.Append(stateErr, err)

	err = r.patchRecoveryStatus(ctx, mariadb, rs)
	stateErr = multierror.Append(stateErr, err)

	if err := stateErr.ErrorOrNil(); err != nil {
		return fmt.Errorf("error getting state: %v", err)
	}

	if rs.isComplete(pods) {
		logger.Info("Recovery status completed")
		if err := r.bootstrap(ctx, rs, pods, clientSet, logger); err != nil {
			return fmt.Errorf("error bootstrapping: %v", err)
		}
		return r.patchRecoveryStatus(ctx, mariadb, rs)
	}

	logger.V(1).Info("Recovery by Pod")
	var recoveryErr *multierror.Error
	err = r.recoveryByPod(ctx, mariadb, pods, rs, clientSet, logger)
	recoveryErr = multierror.Append(recoveryErr, err)

	err = r.patchRecoveryStatus(ctx, mariadb, rs)
	recoveryErr = multierror.Append(recoveryErr, err)

	if err := recoveryErr.ErrorOrNil(); err != nil {
		return fmt.Errorf("error performing recovery: %v", err)
	}

	if !rs.isComplete(pods) {
		return errors.New("recovery status not complete")
	}
	if err := r.bootstrap(ctx, rs, pods, clientSet, logger); err != nil {
		return fmt.Errorf("error bootstrapping: %v", err)
	}
	return r.patchRecoveryStatus(ctx, mariadb, rs)
}

// TODO: handle cases where only a few Pods are not Ready.
// Check status of each node with:
//   - SHOW STATUS LIKE 'wsrep_local_state_comment';
//     The output will display the state of each node, such as "Synced," "Donor," "Joining," "Joined," or "Disconnected."
//     All nodes should ideally be in the "Synced" state.
func (r *GaleraReconciler) recoverPods(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, pods []corev1.Pod,
	clientSet *sqlclient.ClientSet, logger logr.Logger) error {
	notReadyPods, err := r.notReadyPods(ctx, pods, clientSet, logger)
	if err != nil {
		return fmt.Errorf("error getting not Ready Pods: %v", err)
	}
	doneChan := make(chan struct{})

	var wg sync.WaitGroup
	for _, p := range notReadyPods {
		wg.Add(1)
		go func(pod *corev1.Pod) {
			defer wg.Done()

			index, err := statefulset.PodIndex(pod.Name)
			if err != nil {
				logger.Error(err, "error getting Pod index", "pod", pod.Name)
				return
			}
			clientCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			client, err := clientSet.ClientForIndex(clientCtx, *index)
			if err != nil {
				logger.Error(err, "error getting Pod client", "pod", pod.Name)
				return
			}

			if err := r.recoverPod(ctx, mariadb, pod, client, logger); err != nil {
				logger.Error(err, "error recovering Pod", "pod", pod.Name)
			}
		}(&p)
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
	}
}

func (r *GaleraReconciler) recoverPod(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, pod *corev1.Pod,
	clientSet *sqlclient.Client, logger logr.Logger) error {
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

func (r *GaleraReconciler) notReadyPods(ctx context.Context, pods []corev1.Pod,
	clientSet *sqlclient.ClientSet, logger logr.Logger) ([]corev1.Pod, error) {
	var notReadyPods []corev1.Pod
	for _, p := range pods {
		if !pod.PodReady(&p) {
			notReadyPods = append(notReadyPods, p)
			continue
		}
		index, err := statefulset.PodIndex(p.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting Pod '%s' index: %v", p.Name, err)
		}

		clientCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		client, err := clientSet.ClientForIndex(clientCtx, *index)
		if err != nil {
			logger.Error(err, "error getting client", "pod", p.Name)
			notReadyPods = append(notReadyPods, p)
			continue
		}

		state, err := client.GaleraLocalState(clientCtx)
		if err != nil {
			logger.Error(err, "error getting local state", "pod", p.Name)
			notReadyPods = append(notReadyPods, p)
			continue
		}
		if state != "Synced" {
			notReadyPods = append(notReadyPods, p)
		}
	}
	return notReadyPods, nil
}

func (r *GaleraReconciler) stateByPod(ctx context.Context, pods []corev1.Pod, rs *recoveryStatus,
	clientSet *agentClientSet, logger logr.Logger) error {
	doneChan := make(chan struct{})
	errChan := make(chan error)

	var wg sync.WaitGroup
	for _, pod := range pods {
		if _, ok := rs.state(pod.Name); ok {
			logger.V(1).Info("Skipping Pod state", "pod", pod.Name)
			continue
		}

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

			stateCtx, cancelState := context.WithTimeout(ctx, 30*time.Second)
			defer cancelState()
			if err = pollUntilSucessWithTimeout(stateCtx, logger, func(ctx context.Context) error {
				galeraState, err := client.GaleraState.Get(ctx)
				if err != nil {
					return err
				}

				rs.setState(pod.Name, galeraState)
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

func (r *GaleraReconciler) recoveryByPod(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, pods []corev1.Pod, rs *recoveryStatus,
	clientSet *agentClientSet, logger logr.Logger) error {
	doneChan := make(chan struct{})
	errChan := make(chan error)

	var wg sync.WaitGroup
	for _, pod := range pods {
		if _, ok := rs.recovered(pod.Name); ok {
			logger.V(1).Info("Skipping Pod recovery", "pod", pod.Name)
			continue
		}

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

			logger.V(1).Info("Enabling recovery", "pod", pod.Name)
			enableCtx, cancelEnable := context.WithTimeout(ctx, 30*time.Second)
			defer cancelEnable()
			if err = pollUntilSucessWithTimeout(enableCtx, logger, func(ctx context.Context) error {
				return client.Recovery.Enable(ctx)
			}); err != nil {
				errChan <- fmt.Errorf("error enabling recovery in Pod '%s': %v", pod.Name, err)
				return
			}

			deleteCtx, cancelDelete := context.WithTimeout(ctx, 3*time.Minute)
			defer cancelDelete()
			go func() {
				ticker := time.NewTicker(30 * time.Second)
				for {
					select {
					case <-deleteCtx.Done():
						return
					case <-ticker.C:
						logger.V(1).Info("Deleting Pod", "pod", pod.Name)
						if err := r.Delete(ctx, pod); err != nil {
							logger.V(1).Error(err, "Error deleting Pod", "pod", pod.Name)
						}
					}
				}
			}()

			logger.V(1).Info("Performing recovery", "pod", pod.Name)
			recoveryCtx, cancelRecovery := context.WithTimeout(ctx, mariadb.Spec.Galera.Recovery.PodRecoveryTimeoutOrDefault())
			defer cancelRecovery()
			if err = pollUntilSucessWithTimeout(recoveryCtx, logger, func(ctx context.Context) error {
				bootstrap, err := client.Recovery.Start(ctx)
				if err != nil {
					return err
				}

				rs.setRecovered(pod.Name, bootstrap)
				return nil
			}); err != nil {
				errChan <- fmt.Errorf("error performing recovery in Pod '%s': %v", pod.Name, err)
				return
			}
			cancelDelete()

			logger.V(1).Info("Disabling recovery", "pod", pod.Name)
			disableCtx, cancelDisable := context.WithTimeout(ctx, 30*time.Second)
			defer cancelDisable()
			if err = pollUntilSucessWithTimeout(disableCtx, logger, func(ctx context.Context) error {
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

func (r *GaleraReconciler) bootstrap(ctx context.Context, rs *recoveryStatus, pods []corev1.Pod,
	clientSet *agentClientSet, logger logr.Logger) error {
	src, err := rs.bootstrapSource(pods)
	if err != nil {
		return fmt.Errorf("error getting bootstrap source: %v", err)
	}
	logger.Info("Bootstrapping cluster", "pod", src.pod.Name)

	idx, err := statefulset.PodIndex(src.pod.Name)
	if err != nil {
		return fmt.Errorf("error getting index for Pod '%s': %v", src.pod.Name, err)
	}
	client, err := clientSet.clientForIndex(*idx)
	if err != nil {
		return fmt.Errorf("error getting client for Pod '%s': %v", src.pod, err)
	}

	bootstrapCtx, cancelBootstrap := context.WithTimeout(ctx, 30*time.Second)
	defer cancelBootstrap()
	if err = pollUntilSucessWithTimeout(bootstrapCtx, logger, func(ctx context.Context) error {
		return client.Bootstrap.Enable(ctx, src.bootstrap)
	}); err != nil {
		return fmt.Errorf("error enabling bootstrap in Pod '%s': %v", src.pod.Name, err)
	}

	deleteCtx, cancelDelete := context.WithTimeout(ctx, 30*time.Second)
	defer cancelDelete()
	if err = pollUntilSucessWithTimeout(deleteCtx, logger, func(ctx context.Context) error {
		return r.Delete(ctx, src.pod)
	}); err != nil {
		return fmt.Errorf("error deleting Pod '%s': %v", src.pod.Name, err)
	}

	rs.setBootstrapping()
	return nil
}

func (r *GaleraReconciler) patchRecoveryStatus(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, rs *recoveryStatus) error {
	return r.patchStatus(ctx, mdb, func(mdbStatus *mariadbv1alpha1.MariaDBStatus) {
		mdbStatus.GaleraRecovery = rs.galeraRecoveryStatus()
	})
}

func pollUntilSucessWithTimeout(ctx context.Context, logger logr.Logger, fn func(ctx context.Context) error) error {
	// TODO: bump apimachinery and migrate to PollUntilContextTimeout.
	// See: https://pkg.go.dev/k8s.io/apimachinery@v0.27.2/pkg/util/wait#PollUntilContextTimeout
	if err := wait.PollImmediateUntilWithContext(ctx, 1*time.Second, func(ctx context.Context) (bool, error) {
		err := fn(ctx)
		if err != nil {
			logger.V(1).Error(err, "Error polling")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}
