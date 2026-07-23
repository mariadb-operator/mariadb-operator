package init

import (
	"context"
	"fmt"
	"os"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/filemanager"
	jobpkg "github.com/mariadb-operator/mariadb-operator/v26/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/log"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	replicationresources "github.com/mariadb-operator/mariadb-operator/v26/pkg/replication"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const replicationConfigFile = "0-replication.cnf"

var replicationCommand = &cobra.Command{
	Use:   "replication",
	Short: "Replication.",
	Long:  "Init replication instances.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("Starting replication init")

		ctx, cancel := newContext()
		defer cancel()

		env, err := environment.GetPodEnv(ctx)
		if err != nil {
			logger.Error(err, "Error getting environment variables")
			os.Exit(1)
		}
		fileManager, err := filemanager.NewFileManager(configDir, stateDir)
		if err != nil {
			logger.Error(err, "Error creating file manager")
			os.Exit(1)
		}
		k8sClient, err := getK8sClient()
		if err != nil {
			logger.Error(err, "Error getting Kubernetes client")
			os.Exit(1)
		}
		podIndex, err := statefulset.PodIndex(env.PodName)
		if err != nil {
			logger.Error(err, "error getting index from Pod", "pod", env.PodName)
			os.Exit(1)
		}

		if err := createReplicationConfig(env, fileManager); err != nil {
			logger.Error(err, "error creating replication configuration")
			os.Exit(1)
		}

		key := types.NamespacedName{
			Name:      env.MariadbName,
			Namespace: env.PodNamespace,
		}
		var mdb mariadbv1alpha1.MariaDB
		if err := k8sClient.Get(ctx, key, &mdb); err != nil {
			logger.Error(err, "Error getting MariaDB")
			os.Exit(1)
		}

		recovering, err := waitForReplicaRecovery(ctx, env, &mdb, *podIndex, k8sClient)
		if err != nil {
			logger.Error(err, "error waiting for replica recovery")
			os.Exit(1)
		}
		if !recovering {
			restored, err := isReplicaRestoreComplete(ctx, &mdb, *podIndex, k8sClient)
			if err != nil {
				logger.Error(err, "error checking replica restore state. Keeping GTID state file")
			} else if !restored {
				if err := cleanupStateFile(fileManager, replicationresources.MariaDBOperatorFileName); err != nil {
					logger.Error(err, "error cleaning up GTID state file")
					os.Exit(1)
				}
			}
		}
		if err := cleanupReplicaState(fileManager, &mdb, *podIndex); err != nil {
			logger.Error(err, "error cleaning up replica state")
			os.Exit(1)
		}
		logger.Info("Replication init done")
	},
}

func createReplicationConfig(env *environment.PodEnvironment, fileManager *filemanager.FileManager) error {
	replConfig, err := replication.NewReplicationConfig(env)
	if err != nil {
		return err
	}
	logger.Info("Configuring replication")
	return fileManager.WriteConfigFile(replicationConfigFile, replConfig)
}

func waitForReplicaRecovery(ctx context.Context, env *environment.PodEnvironment, mdb *mariadbv1alpha1.MariaDB,
	podIndex int, client ctrlclient.Client) (bool, error) {
	if !mdb.IsReplicaBeingRecovered(env.PodName) {
		return false, nil
	}
	logger.Info("Waiting for replica recovery")
	key := ctrlclient.ObjectKeyFromObject(mdb)

	recovering := true
	if err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(context.Context) (bool, error) {
		var mariadb mariadbv1alpha1.MariaDB
		if err := client.Get(ctx, key, &mariadb); err != nil {
			return false, fmt.Errorf("error getting MariaDB: %v", err)
		}
		if !mariadb.IsReplicaBeingRecovered(env.PodName) {
			recovering = false
			return true, nil
		}
		restored, err := isReplicaRestoreComplete(ctx, &mariadb, podIndex, client)
		if err != nil {
			logger.V(1).Info("Error checking replica restore Job", "err", err)
			return false, nil
		}
		if restored {
			logger.Info("Replica restore Job completed. Starting to get replication configured")
			return true, nil
		}
		logger.V(1).Info("Replica is being recovered")
		return false, nil
	}); err != nil {
		return false, err
	}
	return recovering, nil
}

func isReplicaRestoreComplete(ctx context.Context, mdb *mariadbv1alpha1.MariaDB, podIndex int,
	client ctrlclient.Client) (bool, error) {
	var pvc corev1.PersistentVolumeClaim
	if err := client.Get(ctx, mdb.PVCKey(builder.StorageVolume, podIndex), &pvc); err != nil {
		return false, fmt.Errorf("error getting storage PVC: %v", err)
	}
	completedUID := mdb.Annotations[metadata.ReplicaRecoveryCompletedPVCUIDAnnotationKey(podIndex)]
	if completedUID != "" && completedUID == string(pvc.UID) {
		return true, nil
	}

	var job batchv1.Job
	if err := client.Get(ctx, mdb.PhysicalBackupInitJobKey(podIndex), &job); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("error getting restore Job: %v", err)
	}
	if !jobpkg.IsJobComplete(&job) {
		return false, nil
	}
	if jobPVCUID := job.Annotations[metadata.InitJobStoragePVCUIDAnnotation]; jobPVCUID != "" {
		return jobPVCUID == string(pvc.UID), nil
	}
	if !job.CreationTimestamp.IsZero() && !pvc.CreationTimestamp.IsZero() &&
		job.CreationTimestamp.Time.Before(pvc.CreationTimestamp.Time) {
		return false, nil
	}
	return true, nil
}

// Cleanup previous replica state files during initialization
func cleanupReplicaState(fm *filemanager.FileManager, mdb *mariadbv1alpha1.MariaDB, podIndex int) error {
	if mdb.HasConfiguredReplication() || mdb.IsSwitchingPrimary() {
		return nil
	}
	logger.Info("Cleaning up replica state")

	for _, file := range []string{replicationresources.MasterInfoFileName, replicationresources.RelayLogFileName} {
		if err := cleanupStateFile(fm, file); err != nil {
			return err
		}
	}
	return nil
}
