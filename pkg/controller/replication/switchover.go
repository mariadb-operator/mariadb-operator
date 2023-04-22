package replication

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/conditions"
	mariadbclient "github.com/mariadb-operator/mariadb-operator/pkg/mariadb"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type switchoverPhase struct {
	name      string
	reconcile func(context.Context, *mariadbv1alpha1.MariaDB, *mariadbClientSet) error
}

func (r *ReplicationReconciler) reconcileSwitchover(ctx context.Context, req *reconcileRequest) error {
	if req.mariadb.Status.CurrentPrimaryPodIndex == nil {
		return nil
	}
	if req.mariadb.Spec.Replication.PrimaryPodIndex == *req.mariadb.Status.CurrentPrimaryPodIndex {
		return nil
	}
	stsReady, err := r.statefulSetReady(ctx, req.mariadb)
	if err != nil {
		return fmt.Errorf("error checking StatefulSet readiness: %v", err)
	}
	if !stsReady {
		return fmt.Errorf("StatefulSet not ready: %v", err)
	}

	if err := r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		var errBundle *multierror.Error
		err := conditions.SetReadySwitchingPrimary(&req.mariadb.Status, req.mariadb)
		errBundle = multierror.Append(errBundle, err)

		err = conditions.SetPrimarySwitchedInProgress(&req.mariadb.Status, req.mariadb)
		errBundle = multierror.Append(errBundle, err)
		return errBundle.ErrorOrNil()
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	logger := log.FromContext(ctx)
	logger.Info(
		"switching primary",
		"fromIndex",
		*req.mariadb.Status.CurrentPrimaryPodIndex,
		"toIndex",
		req.mariadb.Spec.Replication.PrimaryPodIndex,
	)

	phases := []switchoverPhase{
		{
			name:      "lock current primary tables",
			reconcile: r.lockCurrentPrimary,
		},
		{
			name:      "wait for replica sync",
			reconcile: r.waitForReplicaSync,
		},
		{
			name:      "prepare new primary",
			reconcile: r.prepareNewPrimary,
		},
		{
			name:      "connect replicas to new primary",
			reconcile: r.connectReplicasToNewPrimary,
		},
		{
			name:      "change current primary to replica",
			reconcile: r.changeCurrentPrimaryToReplica,
		},
	}

	for _, p := range phases {
		logger.Info(p.name)
		if err := p.reconcile(ctx, req.mariadb, req.clientSet); err != nil {
			return fmt.Errorf("error in '%s' switchover reconcile phase: %v", p.name, err)
		}
	}

	if err := r.patchStatus(ctx, req.mariadb, func(status *mariadbv1alpha1.MariaDBStatus) error {
		status.CurrentPrimaryPodIndex = &req.mariadb.Spec.Replication.PrimaryPodIndex
		conditions.SetPrimarySwitchedComplete(&req.mariadb.Status)
		return nil
	}); err != nil {
		return fmt.Errorf("error patching MariaDB status: %v", err)
	}
	logger.Info("primary switchover completed")

	return nil
}

func (r *ReplicationReconciler) statefulSetReady(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (bool, error) {
	var sts appsv1.StatefulSet
	stsKey := types.NamespacedName{
		Name:      mariadb.Name,
		Namespace: mariadb.Namespace,
	}
	if err := r.Get(ctx, stsKey, &sts); err != nil {
		return false, fmt.Errorf("error getting StatefulSet '%s': %v", stsKey.Name, err)
	}
	if sts.Status.ReadyReplicas != sts.Status.Replicas {
		return false, nil
	}
	return true, nil
}

func (r *ReplicationReconciler) lockCurrentPrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *mariadbClientSet) error {
	client, err := clientSet.currentPrimaryClient()
	if err != nil {
		return fmt.Errorf("error getting current primary client: %v", err)
	}
	if err := client.LockTablesWithReadLock(ctx); err != nil {
		return fmt.Errorf("error locking tables in primary: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) waitForReplicaSync(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *mariadbClientSet) error {
	client, err := clientSet.currentPrimaryClient()
	if err != nil {
		return fmt.Errorf("error getting current primary client: %v", err)
	}
	primaryGtid, err := client.GlobalVar(ctx, "gtid_binlog_pos")
	if err != nil {
		return fmt.Errorf("error getting primary GTID: %v", err)
	}

	var wg sync.WaitGroup
	doneChan := make(chan struct{})
	errChan := make(chan error)

	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if i == *mariadb.Status.CurrentPrimaryPodIndex {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			replClient, err := clientSet.replicaClient(i)
			if err != nil {
				errChan <- fmt.Errorf("error getting replica '%d' client: %v", i, err)
				return
			}

			log.FromContext(ctx).V(1).Info("syncing replica with primary GTID", "replica", i, "gtid", primaryGtid)
			timeout := 30 * time.Second
			if mariadb.Spec.Replication.Timeout != nil {
				timeout = mariadb.Spec.Replication.Timeout.Duration
			}
			if err := replClient.WaitForReplicaGtid(ctx, primaryGtid, timeout); err != nil {
				errChan <- fmt.Errorf("error waiting for GTID '%s' in replica '%d'", err, i)
			}
		}(i)
	}
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneChan:
		break
	case err := <-errChan:
		return err
	}
	return nil
}

func (r *ReplicationReconciler) prepareNewPrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *mariadbClientSet) error {
	client, err := clientSet.newPrimaryClient()
	if err != nil {
		return fmt.Errorf("error getting new primary client: %v", err)
	}

	config := primaryConfig{
		mariadb: mariadb,
		client:  client,
		ordinal: mariadb.Spec.Replication.PrimaryPodIndex,
	}
	if err := r.configurePrimary(ctx, &config); err != nil {
		return fmt.Errorf("error confguring new primary vars: %v", err)
	}
	return nil
}

func (r *ReplicationReconciler) connectReplicasToNewPrimary(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *mariadbClientSet) error {
	var replSecret corev1.Secret
	if err := r.Get(ctx, replPasswordKey(mariadb), &replSecret); err != nil {
		return fmt.Errorf("error getting replication password Secret: %v", err)
	}

	gtid, err := mariadb.Spec.Replication.Gtid.MariaDBFormat()
	if err != nil {
		return fmt.Errorf("error getting GTID: %v", err)
	}
	changeMasterOpts := &mariadbclient.ChangeMasterOpts{
		Connection: ConnectionName,
		Host: statefulset.PodFQDN(
			mariadb.ObjectMeta,
			mariadb.Spec.Replication.PrimaryPodIndex,
		),
		User:     ReplUser,
		Password: string(replSecret.Data[PasswordSecretKey]),
		Gtid:     gtid,
	}

	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if i == *mariadb.Status.CurrentPrimaryPodIndex || i == mariadb.Spec.Replication.PrimaryPodIndex {
			continue
		}
		client, err := clientSet.replicaClient(i)
		if err != nil {
			return fmt.Errorf("error getting replica '%d' client: %v", i, err)
		}
		config := replicaConfig{
			mariadb:          mariadb,
			client:           client,
			changeMasterOpts: changeMasterOpts,
			ordinal:          i,
		}
		if err := r.configureReplica(ctx, &config); err != nil {
			return fmt.Errorf("error configuring replica vars in replica '%d': %v", err, i)
		}
	}
	return nil
}

func (r *ReplicationReconciler) changeCurrentPrimaryToReplica(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	clientSet *mariadbClientSet) error {
	client, err := clientSet.currentPrimaryClient()
	if err != nil {
		return fmt.Errorf("error getting current primary client: %v", err)
	}
	var replSecret corev1.Secret
	if err := r.Get(ctx, replPasswordKey(mariadb), &replSecret); err != nil {
		return fmt.Errorf("error getting replication password Secret: %v", err)
	}

	gtid, err := mariadb.Spec.Replication.Gtid.MariaDBFormat()
	if err != nil {
		return fmt.Errorf("error getting GTID: %v", err)
	}

	config := replicaConfig{
		mariadb: mariadb,
		client:  client,
		changeMasterOpts: &mariadbclient.ChangeMasterOpts{
			Connection: ConnectionName,
			Host: statefulset.PodFQDN(
				mariadb.ObjectMeta,
				mariadb.Spec.Replication.PrimaryPodIndex,
			),
			User:     ReplUser,
			Password: string(replSecret.Data[PasswordSecretKey]),
			Gtid:     gtid,
		},
		ordinal: *mariadb.Status.CurrentPrimaryPodIndex,
	}
	if err := r.configureReplica(ctx, &config); err != nil {
		return fmt.Errorf("error configuring replica vars in current primary: %v", err)
	}
	return nil
}
