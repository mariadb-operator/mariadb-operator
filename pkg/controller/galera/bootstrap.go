package galera

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	galeraclient "github.com/mariadb-operator/mariadb-operator/pkg/galera/client"
	galerarecovery "github.com/mariadb-operator/mariadb-operator/pkg/galera/recovery"
	"github.com/mariadb-operator/mariadb-operator/pkg/sql"
	sqlClientSet "github.com/mariadb-operator/mariadb-operator/pkg/sqlset"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

type bootstrapClusterOpts struct {
	bootstrapPodKey types.NamespacedName
	agentClientSet  *agentClientSet
	sqlClientSet    *sqlClientSet.ClientSet
}

func (r *GaleraReconciler) bootstrapCluster(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	bopts *bootstrapClusterOpts, logger logr.Logger) error {
	podKeys := r.getPodKeys(mariadb, bopts.bootstrapPodKey)

	galera := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{})
	specRecovery := ptr.Deref(galera.Recovery, mariadbv1alpha1.GaleraRecovery{})

	syncTimeout := ptr.Deref(specRecovery.PodSyncTimeout, metav1.Duration{Duration: 3 * time.Minute}).Duration
	syncContext, syncCancel := context.WithTimeout(ctx, syncTimeout)
	defer syncCancel()

	for _, key := range podKeys {
		if key.Name == bopts.bootstrapPodKey.Name {
			logger.Info("Enabling bootstrap at Pod", "pod", key.Name)
			podIndex, err := statefulset.PodIndex(key.Name)
			if err != nil {
				return fmt.Errorf("error getting bootstrap Pod index: %v", err)
			}

			agentClient, err := bopts.agentClientSet.clientForIndex(*podIndex)
			if err != nil {
				return fmt.Errorf("error getting agent client for bootstrap Pod: %v", err)
			}
			state, err := agentClient.State.GetGaleraState(ctx)
			if err != nil {
				return fmt.Errorf("error getting bootstrap Pod state: %v", err)
			}
			if err := agentClient.Bootstrap.Enable(ctx, galerarecovery.NewBootstrap(state.UUID, state.Seqno)); err != nil {
				return fmt.Errorf("error enabling bootstrap in Pod: %v", err)
			}

			logger.Info("Restarting bootstrap Pod", "pod", key.Name)
		} else {
			logger.Info("Restarting Pod", "pod", key.Name)
		}

		if err := r.pollUntilPodDeleted(syncContext, key, logger); err != nil {
			return fmt.Errorf("error deleting Pod '%s': %v", key.Name, err)
		}
		if err := r.pollUntilPodSynced(syncContext, key, bopts.sqlClientSet, logger); err != nil {
			return fmt.Errorf("error waiting for Pod '%s' to be synced: %v", key.Name, err)
		}
	}
	return nil
}

func (r *GaleraReconciler) getPodKeys(mariadb *mariadbv1alpha1.MariaDB, bootstrapPodKey types.NamespacedName) []types.NamespacedName {
	podKeys := []types.NamespacedName{
		bootstrapPodKey,
	}
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		name := statefulset.PodName(mariadb.ObjectMeta, i)
		if name == bootstrapPodKey.Name {
			continue
		}
		podKeys = append(podKeys, types.NamespacedName{
			Name:      name,
			Namespace: mariadb.Namespace,
		})
	}
	return podKeys
}

func (r *GaleraReconciler) pollUntilPodDeleted(ctx context.Context, podKey types.NamespacedName, logger logr.Logger) error {
	return pollUntilSucessWithTimeout(ctx, logger, func(ctx context.Context) error {
		var pod corev1.Pod
		if err := r.Get(ctx, podKey, &pod); err != nil {
			return fmt.Errorf("error getting Pod '%s': %v", podKey.Name, err)
		}
		if err := r.Delete(ctx, &pod); err != nil {
			return fmt.Errorf("error deleting Pod '%s': %v", podKey.Name, err)
		}
		return nil
	})
}

func (r *GaleraReconciler) pollUntilPodSynced(ctx context.Context, podKey types.NamespacedName, sqlClientSet *sqlClientSet.ClientSet,
	logger logr.Logger) error {
	return pollUntilSucessWithTimeout(ctx, logger, func(ctx context.Context) error {
		var pod corev1.Pod
		if err := r.Get(ctx, podKey, &pod); err != nil {
			return fmt.Errorf("error getting Pod '%s': %v", podKey.Name, err)
		}

		podIndex, err := statefulset.PodIndex(podKey.Name)
		if err != nil {
			return fmt.Errorf("error getting Pod index: %v", err)
		}
		sqlClient, err := sqlClientSet.ClientForIndex(ctx, *podIndex, sql.WithTimeout(5*time.Second))
		if err != nil {
			return fmt.Errorf("errpr getting SQL client: %v", err)
		}

		synced, err := galeraclient.IsPodSynced(ctx, sqlClient)
		if err != nil {
			return fmt.Errorf("error checking Pod sync: %v", err)
		}
		if !synced {
			return errors.New("Pod not synced")
		}
		return nil
	})
}
