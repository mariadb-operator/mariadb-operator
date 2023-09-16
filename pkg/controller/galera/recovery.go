package galera

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"github.com/mariadb-operator/agent/pkg/client"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	sqlclient "github.com/mariadb-operator/mariadb-operator/pkg/client"
	"github.com/mariadb-operator/mariadb-operator/pkg/pod"
	mariadbpod "github.com/mariadb-operator/mariadb-operator/pkg/pod"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *GaleraReconciler) reconcileRecovery(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, sts *appsv1.StatefulSet,
	logger logr.Logger) error {
	pods, err := r.pods(ctx, mariadb)
	if err != nil {
		return fmt.Errorf("error getting Pods: %v", err)
	}
	agentClientSet, err := r.newAgentClientSet(mariadb, client.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("error getting agent client: %v", err)
	}
	sqlClientSet := sqlclient.NewClientSet(mariadb, r.refResolver)
	defer sqlClientSet.Close()

	if sts.Status.ReadyReplicas == 0 {
		return r.recoverCluster(ctx, mariadb, pods, agentClientSet, logger.WithName("cluster"))
	}
	return r.recoverPods(ctx, mariadb, pods, sqlClientSet, logger.WithName("pod"))
}

func (r *GaleraReconciler) recoverCluster(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, pods []corev1.Pod,
	clientSet *agentClientSet, logger logr.Logger) error {
	rs := newRecoveryStatus(mariadb)

	if rs.isBootstrapping() {
		if rs.bootstrapTimeout(mariadb) {
			logger.Info("Galera cluster bootstrap timed out. Resetting recovery status")
			r.recorder.Event(mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonGaleraClusterBootstrapTimeout,
				"Galera cluster bootstrap timed out")

			rs.reset()
			return r.patchRecoveryStatus(ctx, mariadb, rs)
		}
		return nil
	}

	logger.V(1).Info("State by Pod")
	var stateErr *multierror.Error
	err := r.stateByPod(ctx, mariadb, pods, rs, clientSet, logger)
	stateErr = multierror.Append(stateErr, err)

	err = r.patchRecoveryStatus(ctx, mariadb, rs)
	stateErr = multierror.Append(stateErr, err)

	if err := stateErr.ErrorOrNil(); err != nil {
		return fmt.Errorf("error getting state: %v", err)
	}

	src, err := rs.bootstrapSource(pods)
	if err != nil {
		logger.V(1).Info("Error getting bootstrap source", "err", err)
	}
	if src != nil {
		if err := r.bootstrap(ctx, src, rs, mariadb, clientSet, logger); err != nil {
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

	src, err = rs.bootstrapSource(pods)
	if err != nil {
		return fmt.Errorf("error getting bootstrap source: %v", err)
	}
	if err := r.bootstrap(ctx, src, rs, mariadb, clientSet, logger); err != nil {
		return fmt.Errorf("error bootstrapping: %v", err)
	}
	return r.patchRecoveryStatus(ctx, mariadb, rs)
}

func (r *GaleraReconciler) recoverPods(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, pods []corev1.Pod,
	clientSet *sqlclient.ClientSet, logger logr.Logger) error {
	doneChan := make(chan struct{})

	var wg sync.WaitGroup
	for _, p := range r.notReadyPods(pods) {
		wg.Add(1)
		podKey := ctrlclient.ObjectKeyFromObject(&p)
		go func(podKey types.NamespacedName) {
			defer wg.Done()

			if err := r.recoverPod(ctx, mariadb, podKey, clientSet, logger); err != nil {
				logger.Error(err, "Error recovering Pod", "pod", podKey.Name)
			}
		}(podKey)
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

func (r *GaleraReconciler) recoverPod(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, podKey types.NamespacedName,
	clientSet *sqlclient.ClientSet, logger logr.Logger) error {
	syncTimeout := mariadb.Galera().Recovery.PodSyncTimeout.Duration
	syncCtx, cancelSync := context.WithTimeout(ctx, syncTimeout)
	defer cancelSync()

	if err := pollUntilSucessWithTimeout(syncCtx, logger, func(ctx context.Context) error {
		podCtx, cancelPod := context.WithTimeout(ctx, 5*time.Second)
		defer cancelPod()

		var pod corev1.Pod
		if err := r.Get(podCtx, podKey, &pod); err != nil {
			return fmt.Errorf("error getting Pod '%s': %v", podKey.Name, err)
		}
		if mariadbpod.PodReady(&pod) {
			logger.Info("Pod became Ready. Stopping recovery", "pod", podKey.Name)
			return nil
		}

		index, err := statefulset.PodIndex(pod.Name)
		if err != nil {
			return fmt.Errorf("error getting index for Pod '%s': %v", podKey.Name, err)
		}
		client, err := clientSet.ClientForIndex(podCtx, *index)
		if err != nil {
			return fmt.Errorf("error getting client for Pod '%s': %v", podKey.Name, err)
		}

		state, err := client.GaleraLocalState(podCtx)
		if err != nil {
			return fmt.Errorf("error getting Pod '%s' state: %v", podKey.Name, err)
		}
		if state != "Synced" {
			return fmt.Errorf("Pod '%s' in non Synced state: '%s'", podKey.Name, state)
		}
		return nil
	}); err != nil {
		logger.Error(err, "Timeout waiting for Pod to be Synced. Deleting Pod", "pod", podKey.Name, "timeout", syncTimeout.String())
		r.recorder.Eventf(mariadb, corev1.EventTypeWarning, mariadbv1alpha1.ReasonGaleraPodSyncTimeout,
			"Timeout waiting for Pod '%s' to be Synced", podKey.Name)

		podCtx, cancelPod := context.WithTimeout(ctx, 5*time.Second)
		defer cancelPod()
		var pod corev1.Pod
		if err := r.Get(podCtx, podKey, &pod); err != nil {
			return fmt.Errorf("error getting Pod '%s': %v", podKey.Name, err)
		}

		deleteCtx, cancelDelete := context.WithTimeout(ctx, 30*time.Second)
		defer cancelDelete()
		if err := pollUntilSucessWithTimeout(deleteCtx, logger, func(ctx context.Context) error {
			return r.Delete(ctx, &pod)
		}); err != nil {
			return err
		}
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
			return nil, fmt.Errorf("error getting Pod '%s': %v", key.Name, err)
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

func (r *GaleraReconciler) notReadyPods(pods []corev1.Pod) []corev1.Pod {
	var notReadyPods []corev1.Pod
	for _, p := range pods {
		if pod.PodReady(&p) {
			continue
		}
		notReadyPods = append(notReadyPods, p)
	}
	return notReadyPods
}

func (r *GaleraReconciler) stateByPod(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, pods []corev1.Pod, rs *recoveryStatus,
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
		go func(i int, pod corev1.Pod) {
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

				logger.Info("Galera state fetched in Pod", "pod", pod.Name)
				r.recorder.Eventf(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonGaleraPodStateFetched,
					"Galera state fetched in Pod '%s'", pod.Name)
				rs.setState(pod.Name, galeraState)
				return nil
			}); err != nil {
				errChan <- fmt.Errorf("error getting Galera state for Pod '%s': %v", pod.Name, err)
			}
		}(*i, pod)
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
		go func(i int, pod corev1.Pod) {
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
						if err := r.Delete(ctx, &pod); err != nil {
							logger.V(1).Info("Error deleting Pod", "pod", pod.Name, "err", err)
						}
					}
				}
			}()

			logger.V(1).Info("Performing recovery", "pod", pod.Name)
			recoveryCtx, cancelRecovery := context.WithTimeout(ctx, mariadb.Galera().Recovery.PodRecoveryTimeout.Duration)
			defer cancelRecovery()
			if err = pollUntilSucessWithTimeout(recoveryCtx, logger, func(ctx context.Context) error {
				bootstrap, err := client.Recovery.Start(ctx)
				if err != nil {
					return err
				}

				logger.Info("Recovered Galera sequence in Pod", "pod", pod.Name)
				r.recorder.Eventf(mariadb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonGaleraPodRecovered,
					"Recovered Galera sequence in Pod '%s'", pod.Name)
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
		}(*i, pod)
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

func (r *GaleraReconciler) bootstrap(ctx context.Context, src *bootstrapSource, rs *recoveryStatus, mdb *mariadbv1alpha1.MariaDB,
	clientSet *agentClientSet, logger logr.Logger) error {
	logger.Info("Bootstrapping cluster", "pod", src.pod.Name)
	r.recorder.Eventf(mdb, corev1.EventTypeNormal, mariadbv1alpha1.ReasonGaleraClusterBootstrap,
		"Bootstrapping Galera cluster in Pod '%s'", src.pod.Name)

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
	if err := pollUntilSucessWithTimeout(deleteCtx, logger, func(ctx context.Context) error {
		return r.Delete(ctx, src.pod)
	}); err != nil {
		return err
	}

	rs.setBootstrapping(src.pod.Name)
	return nil
}

func (r *GaleraReconciler) patchRecoveryStatus(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, rs *recoveryStatus) error {
	return r.patchStatus(ctx, mdb, func(mdbStatus *mariadbv1alpha1.MariaDBStatus) {
		mdbStatus.GaleraRecovery = rs.galeraRecoveryStatus()
	})
}

func pollUntilSucessWithTimeout(ctx context.Context, logger logr.Logger, fn func(ctx context.Context) error) error {
	if err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
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
