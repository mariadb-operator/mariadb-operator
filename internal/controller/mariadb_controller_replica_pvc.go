package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	stsobj "github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const storagePVCUIDAnnotationPrefix = "k8s.mariadb.com/storage-pvc-uid-"
const replicaRecoveryRefreshPVCUIDAnnotationPrefix = "k8s.mariadb.com/replica-recovery-refresh-pvc-uid-"
const replicaRecoveryNodeAnnotationPrefix = "k8s.mariadb.com/replica-recovery-node-"
const replicaRecoveryCompletedPVCUIDAnnotationPrefix = "k8s.mariadb.com/replica-recovery-completed-pvc-uid-"
const initJobStoragePVCUIDAnnotation = "k8s.mariadb.com/init-job-storage-pvc-uid"
const sqlReconcileTokenAnnotation = "k8s.mariadb.com/sql-reconcile-token"

type storagePVCState struct {
	UID               string
	CreationTimestamp metav1.Time
}

type pvcChange struct {
	PodIndex   int
	StoredUID  string
	CurrentUID string
}

func storagePVCUIDAnnotationKey(podIndex int) string {
	return fmt.Sprintf("%s%d", storagePVCUIDAnnotationPrefix, podIndex)
}

func replicaRecoveryRefreshPVCUIDAnnotationKey(podIndex int) string {
	return fmt.Sprintf("%s%d", replicaRecoveryRefreshPVCUIDAnnotationPrefix, podIndex)
}

func replicaRecoveryNodeAnnotationKey(podIndex int) string {
	return fmt.Sprintf("%s%d", replicaRecoveryNodeAnnotationPrefix, podIndex)
}

func replicaRecoveryCompletedPVCUIDAnnotationKey(podIndex int) string {
	return fmt.Sprintf("%s%d", replicaRecoveryCompletedPVCUIDAnnotationPrefix, podIndex)
}

func storagePVCUIDTrackedAnnotations(annotations map[string]string) map[string]string {
	tracked := make(map[string]string)
	for key, value := range annotations {
		if strings.HasPrefix(key, storagePVCUIDAnnotationPrefix) {
			tracked[key] = value
		}
	}
	return tracked
}

func hasTrackedStoragePVCUIDAnnotations(annotations map[string]string) bool {
	return len(storagePVCUIDTrackedAnnotations(annotations)) > 0
}

func replicaRecoveryRefreshPVCUIDTrackedAnnotations(annotations map[string]string) map[string]string {
	tracked := make(map[string]string)
	for key, value := range annotations {
		if strings.HasPrefix(key, replicaRecoveryRefreshPVCUIDAnnotationPrefix) {
			tracked[key] = value
		}
	}
	return tracked
}

func replicaRecoveryNodeTrackedAnnotations(annotations map[string]string) map[string]string {
	tracked := make(map[string]string)
	for key, value := range annotations {
		if strings.HasPrefix(key, replicaRecoveryNodeAnnotationPrefix) {
			tracked[key] = value
		}
	}
	return tracked
}

func replicaRecoveryCompletedPVCUIDTrackedAnnotations(annotations map[string]string) map[string]string {
	tracked := make(map[string]string)
	for key, value := range annotations {
		if strings.HasPrefix(key, replicaRecoveryCompletedPVCUIDAnnotationPrefix) {
			tracked[key] = value
		}
	}
	return tracked
}

func desiredStoragePVCUIDAnnotations(replicas int32, pvcUIDs map[int]string) map[string]string {
	annotations := make(map[string]string)
	for i := 0; i < int(replicas); i++ {
		if uid := pvcUIDs[i]; uid != "" {
			annotations[storagePVCUIDAnnotationKey(i)] = uid
		}
	}
	return annotations
}

func shouldSyncStoragePVCUIDAnnotations(current map[string]string, desired map[string]string) bool {
	currentTracked := storagePVCUIDTrackedAnnotations(current)
	if len(currentTracked) != len(desired) {
		return true
	}
	for key, value := range desired {
		if currentTracked[key] != value {
			return true
		}
	}
	return false
}

func managedSQLRefreshToken(mariadb *mariadbv1alpha1.MariaDB, pvcUIDs map[int]string) (string, bool) {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return "", false
	}

	primary := *mariadb.Status.CurrentPrimaryPodIndex
	currentUID := pvcUIDs[primary]
	if currentUID == "" {
		return "", false
	}

	storedUID, _ := storedStoragePVCUID(mariadb.Annotations, primary)
	return currentUID, currentUID != storedUID
}

func storedStoragePVCUID(annotations map[string]string, podIndex int) (string, bool) {
	if annotations == nil {
		return "", false
	}
	uid, ok := annotations[storagePVCUIDAnnotationKey(podIndex)]
	return uid, ok
}

func storedReplicaRecoveryRefreshPVCUID(annotations map[string]string, podIndex int) (string, bool) {
	if annotations == nil {
		return "", false
	}
	uid, ok := annotations[replicaRecoveryRefreshPVCUIDAnnotationKey(podIndex)]
	return uid, ok
}

func storedReplicaRecoveryNode(annotations map[string]string, podIndex int) (string, bool) {
	if annotations == nil {
		return "", false
	}
	node, ok := annotations[replicaRecoveryNodeAnnotationKey(podIndex)]
	return node, ok
}

func storedReplicaRecoveryCompletedPVCUID(annotations map[string]string, podIndex int) (string, bool) {
	if annotations == nil {
		return "", false
	}
	uid, ok := annotations[replicaRecoveryCompletedPVCUIDAnnotationKey(podIndex)]
	return uid, ok
}

func isReplicaRecoveryCompletedForPVC(annotations map[string]string, podIndex int, pvcState storagePVCState) bool {
	if pvcState.UID == "" {
		return false
	}
	completedUID, ok := storedReplicaRecoveryCompletedPVCUID(annotations, podIndex)
	return ok && completedUID == pvcState.UID
}

func (r *MariaDBReconciler) getStoragePVCStates(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB) (map[int]storagePVCState, error) {
	pvcStates := make(map[int]storagePVCState, int(mariadb.Spec.Replicas))
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		pvcState, ok, err := r.getStoragePVCState(ctx, mariadb, i)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		pvcStates[i] = pvcState
	}
	return pvcStates, nil
}

func (r *MariaDBReconciler) getStoragePVCState(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB, podIndex int) (storagePVCState, bool, error) {
	var pvc corev1.PersistentVolumeClaim
	pvcKey := mariadb.PVCKey(builder.StorageVolume, podIndex)
	if err := r.Get(ctx, pvcKey, &pvc); err != nil {
		if apierrors.IsNotFound(err) {
			return storagePVCState{}, false, nil
		}
		return storagePVCState{}, false, fmt.Errorf("error getting PVC '%s': %v", pvcKey.Name, err)
	}
	return storagePVCState{
		UID:               string(pvc.UID),
		CreationTimestamp: pvc.CreationTimestamp,
	}, true, nil
}

func (r *MariaDBReconciler) getStoragePVCUIDs(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB) (map[int]string, error) {
	pvcStates, err := r.getStoragePVCStates(ctx, mariadb)
	if err != nil {
		return nil, err
	}

	pvcUIDs := make(map[int]string, len(pvcStates))
	for i, state := range pvcStates {
		if state.UID != "" {
			pvcUIDs[i] = state.UID
		}
	}
	return pvcUIDs, nil
}

func getReplicasWithLostPVC(mariadb *mariadbv1alpha1.MariaDB, pvcUIDs map[int]string, logger logr.Logger) []string {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return nil
	}

	var replicas []string
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if i == *mariadb.Status.CurrentPrimaryPodIndex {
			continue
		}

		storedUID, ok := storedStoragePVCUID(mariadb.Annotations, i)
		if !ok || storedUID == "" {
			continue
		}

		replica := stsobj.PodName(mariadb.ObjectMeta, i)
		currentUID := pvcUIDs[i]
		switch {
		case currentUID == "":
			logger.Info("Storage PVC missing, scheduling replica recovery", "replica", replica)
			replicas = append(replicas, replica)
		case currentUID != storedUID:
			logger.Info(
				"Storage PVC recreated, scheduling replica recovery",
				"replica", replica,
				"previous-uid", storedUID,
				"current-uid", currentUID,
			)
			replicas = append(replicas, replica)
		}
	}
	return replicas
}

func getReplicasWithFreshPVCReplicationErrors(mariadb *mariadbv1alpha1.MariaDB, pvcStates map[int]storagePVCState,
	logger logr.Logger) []string {
	if mariadb.Status.CurrentPrimaryPodIndex == nil || mariadb.CreationTimestamp.IsZero() || mariadb.Status.Replication == nil {
		return nil
	}

	var replicas []string
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if i == *mariadb.Status.CurrentPrimaryPodIndex {
			continue
		}

		state, ok := pvcStates[i]
		if !ok || !isRecoverableFreshReplicaPVC(mariadb, pvcStates, i, state) {
			continue
		}

		replica := stsobj.PodName(mariadb.ObjectMeta, i)
		replicaStatus, ok := mariadb.Status.Replication.Replicas[replica]
		if !ok {
			continue
		}

		lastIOErrno := 0
		if replicaStatus.LastIOErrno != nil {
			lastIOErrno = *replicaStatus.LastIOErrno
		}
		lastSQLErrno := 0
		if replicaStatus.LastSQLErrno != nil {
			lastSQLErrno = *replicaStatus.LastSQLErrno
		}
		if lastIOErrno == 0 && lastSQLErrno == 0 {
			continue
		}

		logger.Info(
			"Fresh replica storage has replication errors, scheduling replica recovery",
			"replica", replica,
			"last-io-errno", lastIOErrno,
			"last-sql-errno", lastSQLErrno,
		)
		replicas = append(replicas, replica)
	}
	return replicas
}

func isRecoverableFreshReplicaPVC(mariadb *mariadbv1alpha1.MariaDB, pvcStates map[int]storagePVCState,
	podIndex int, state storagePVCState) bool {
	if state.UID == "" || state.CreationTimestamp.IsZero() {
		return false
	}
	if state.CreationTimestamp.After(mariadb.CreationTimestamp.Time) {
		return true
	}
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return false
	}

	primary := *mariadb.Status.CurrentPrimaryPodIndex
	if primary == podIndex {
		return false
	}

	primaryState, ok := pvcStates[primary]
	if !ok || !isReusableStoragePVCForNewMariaDB(primaryState, mariadb) {
		return false
	}
	return state.CreationTimestamp.After(primaryState.CreationTimestamp.Time)
}

func getPrimaryPVCChange(mariadb *mariadbv1alpha1.MariaDB, pvcUIDs map[int]string) (*pvcChange, bool) {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return nil, false
	}

	primary := *mariadb.Status.CurrentPrimaryPodIndex
	storedUID, ok := storedStoragePVCUID(mariadb.Annotations, primary)
	if !ok || storedUID == "" {
		return nil, false
	}

	currentUID := pvcUIDs[primary]
	if currentUID != "" && currentUID == storedUID {
		return nil, false
	}

	return &pvcChange{
		PodIndex:   primary,
		StoredUID:  storedUID,
		CurrentUID: currentUID,
	}, true
}

func getInitialPrimaryPVCBootstrapCandidate(mariadb *mariadbv1alpha1.MariaDB, pvcStates map[int]storagePVCState) *int {
	if mariadb.Status.CurrentPrimaryPodIndex == nil || mariadb.CreationTimestamp.IsZero() ||
		hasTrackedStoragePVCUIDAnnotations(mariadb.Annotations) {
		return nil
	}

	primary := *mariadb.Status.CurrentPrimaryPodIndex
	if state, ok := pvcStates[primary]; ok && isReusableStoragePVCForNewMariaDB(state, mariadb) {
		return nil
	}

	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if i == primary {
			continue
		}
		state, ok := pvcStates[i]
		if !ok || !isReusableStoragePVCForNewMariaDB(state, mariadb) {
			continue
		}

		candidate := i
		return &candidate
	}
	return nil
}

func isReusableStoragePVCForNewMariaDB(state storagePVCState, mariadb *mariadbv1alpha1.MariaDB) bool {
	return state.UID != "" && !state.CreationTimestamp.IsZero() &&
		state.CreationTimestamp.Time.Before(mariadb.CreationTimestamp.Time)
}

func getReplicaScaleOutStartIndex(mariadb *mariadbv1alpha1.MariaDB, pvcStates map[int]storagePVCState,
	logger logr.Logger) *int {
	if idx := getLostTailReplicaScaleOutStartIndex(mariadb, pvcStates, logger); idx != nil {
		return idx
	}
	if hasTrackedStoragePVCUIDAnnotations(mariadb.Annotations) {
		return nil
	}
	return getFreshTailReplicaScaleOutStartIndex(mariadb, pvcStates, logger)
}

func getLostTailReplicaScaleOutStartIndex(mariadb *mariadbv1alpha1.MariaDB, pvcStates map[int]storagePVCState,
	logger logr.Logger) *int {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return nil
	}
	primary := *mariadb.Status.CurrentPrimaryPodIndex

	startIndex := -1
	for i := 0; i < int(mariadb.Spec.Replicas); i++ {
		if i == primary {
			continue
		}

		storedUID, ok := storedStoragePVCUID(mariadb.Annotations, i)
		if !ok || storedUID == "" {
			continue
		}

		currentUID := pvcStates[i].UID
		if currentUID == "" || currentUID != storedUID {
			startIndex = i
			break
		}
	}
	if startIndex == -1 || primary >= startIndex {
		return nil
	}

	for i := startIndex; i < int(mariadb.Spec.Replicas); i++ {
		if i == primary {
			return nil
		}

		storedUID, ok := storedStoragePVCUID(mariadb.Annotations, i)
		if !ok || storedUID == "" {
			return nil
		}
		currentUID := pvcStates[i].UID
		if currentUID != "" && currentUID == storedUID {
			return nil
		}
	}

	logger.Info("Replica storage tail changed, rescaling StatefulSet to bootstrap replicas", "from-index", startIndex)
	return &startIndex
}

func getFreshTailReplicaScaleOutStartIndex(mariadb *mariadbv1alpha1.MariaDB, pvcStates map[int]storagePVCState,
	logger logr.Logger) *int {
	if mariadb.Status.CurrentPrimaryPodIndex == nil || mariadb.CreationTimestamp.IsZero() {
		return nil
	}
	primary := *mariadb.Status.CurrentPrimaryPodIndex

	for startIndex := 1; startIndex < int(mariadb.Spec.Replicas); startIndex++ {
		if primary >= startIndex {
			continue
		}

		prefixReusable := true
		for i := 0; i < startIndex; i++ {
			state := pvcStates[i]
			if state.UID == "" || !state.CreationTimestamp.Time.Before(mariadb.CreationTimestamp.Time) {
				prefixReusable = false
				break
			}
		}
		if !prefixReusable {
			continue
		}

		hasFreshTail := false
		tailRebuildable := true
		for i := startIndex; i < int(mariadb.Spec.Replicas); i++ {
			if i == primary {
				tailRebuildable = false
				break
			}

			state := pvcStates[i]
			if state.UID == "" {
				hasFreshTail = true
				continue
			}
			if state.CreationTimestamp.Time.Before(mariadb.CreationTimestamp.Time) {
				tailRebuildable = false
				break
			}
			hasFreshTail = true
		}
		if !tailRebuildable || !hasFreshTail {
			continue
		}

		logger.Info("Fresh replica storage tail detected, rescaling StatefulSet to bootstrap replicas", "from-index", startIndex)
		return &startIndex
	}

	return nil
}

func (r *MariaDBReconciler) syncStoragePVCUIDAnnotations(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	pvcUIDs map[int]string) error {
	desiredAnnotations := desiredStoragePVCUIDAnnotations(mariadb.Spec.Replicas, pvcUIDs)

	var current mariadbv1alpha1.MariaDB
	key := client.ObjectKeyFromObject(mariadb)
	if err := r.Get(ctx, key, &current); err != nil {
		return fmt.Errorf("error getting MariaDB: %v", err)
	}
	if refreshToken, shouldRefresh := managedSQLRefreshToken(&current, pvcUIDs); shouldRefresh {
		if err := r.refreshManagedSQLResources(ctx, &current, refreshToken); err != nil {
			return fmt.Errorf("error refreshing managed SQL resources: %v", err)
		}
	}
	if !shouldSyncStoragePVCUIDAnnotations(current.Annotations, desiredAnnotations) {
		return nil
	}

	return r.patch(ctx, &current, func(mdb *mariadbv1alpha1.MariaDB) error {
		if mdb.Annotations == nil {
			mdb.Annotations = map[string]string{}
		}
		for key := range mdb.Annotations {
			if strings.HasPrefix(key, storagePVCUIDAnnotationPrefix) {
				delete(mdb.Annotations, key)
			}
		}
		for key, value := range desiredAnnotations {
			mdb.Annotations[key] = value
		}
		return nil
	})
}

func (r *MariaDBReconciler) syncReplicaRecoveryRefreshPVCUIDAnnotation(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB, podIndex int, pvcUID string) error {
	key := client.ObjectKeyFromObject(mariadb)

	var current mariadbv1alpha1.MariaDB
	if err := r.Get(ctx, key, &current); err != nil {
		return fmt.Errorf("error getting MariaDB: %v", err)
	}
	if current.Annotations != nil && current.Annotations[replicaRecoveryRefreshPVCUIDAnnotationKey(podIndex)] == pvcUID {
		return nil
	}

	return r.patch(ctx, &current, func(mdb *mariadbv1alpha1.MariaDB) error {
		if mdb.Annotations == nil {
			mdb.Annotations = map[string]string{}
		}
		mdb.Annotations[replicaRecoveryRefreshPVCUIDAnnotationKey(podIndex)] = pvcUID
		return nil
	})
}

func (r *MariaDBReconciler) clearReplicaRecoveryRefreshPVCUIDAnnotations(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB) error {
	key := client.ObjectKeyFromObject(mariadb)

	var current mariadbv1alpha1.MariaDB
	if err := r.Get(ctx, key, &current); err != nil {
		return fmt.Errorf("error getting MariaDB: %v", err)
	}
	if len(replicaRecoveryRefreshPVCUIDTrackedAnnotations(current.Annotations)) == 0 {
		return nil
	}

	return r.patch(ctx, &current, func(mdb *mariadbv1alpha1.MariaDB) error {
		for key := range mdb.Annotations {
			if strings.HasPrefix(key, replicaRecoveryRefreshPVCUIDAnnotationPrefix) {
				delete(mdb.Annotations, key)
			}
		}
		return nil
	})
}

func (r *MariaDBReconciler) syncReplicaRecoveryNodeAnnotation(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB, podIndex int, nodeName string) error {
	if nodeName == "" {
		return nil
	}

	key := client.ObjectKeyFromObject(mariadb)

	var current mariadbv1alpha1.MariaDB
	if err := r.Get(ctx, key, &current); err != nil {
		return fmt.Errorf("error getting MariaDB: %v", err)
	}
	if current.Annotations != nil && current.Annotations[replicaRecoveryNodeAnnotationKey(podIndex)] == nodeName {
		return nil
	}

	return r.patch(ctx, &current, func(mdb *mariadbv1alpha1.MariaDB) error {
		if mdb.Annotations == nil {
			mdb.Annotations = map[string]string{}
		}
		mdb.Annotations[replicaRecoveryNodeAnnotationKey(podIndex)] = nodeName
		return nil
	})
}

func (r *MariaDBReconciler) clearReplicaRecoveryNodeAnnotations(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB) error {
	key := client.ObjectKeyFromObject(mariadb)

	var current mariadbv1alpha1.MariaDB
	if err := r.Get(ctx, key, &current); err != nil {
		return fmt.Errorf("error getting MariaDB: %v", err)
	}
	if len(replicaRecoveryNodeTrackedAnnotations(current.Annotations)) == 0 {
		return nil
	}

	return r.patch(ctx, &current, func(mdb *mariadbv1alpha1.MariaDB) error {
		for key := range mdb.Annotations {
			if strings.HasPrefix(key, replicaRecoveryNodeAnnotationPrefix) {
				delete(mdb.Annotations, key)
			}
		}
		return nil
	})
}

func (r *MariaDBReconciler) syncReplicaRecoveryCompletedPVCUIDAnnotation(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB, podIndex int, pvcUID string) error {
	if pvcUID == "" {
		return nil
	}

	key := client.ObjectKeyFromObject(mariadb)

	var current mariadbv1alpha1.MariaDB
	if err := r.Get(ctx, key, &current); err != nil {
		return fmt.Errorf("error getting MariaDB: %v", err)
	}
	if current.Annotations != nil && current.Annotations[replicaRecoveryCompletedPVCUIDAnnotationKey(podIndex)] == pvcUID {
		return nil
	}

	return r.patch(ctx, &current, func(mdb *mariadbv1alpha1.MariaDB) error {
		if mdb.Annotations == nil {
			mdb.Annotations = map[string]string{}
		}
		mdb.Annotations[replicaRecoveryCompletedPVCUIDAnnotationKey(podIndex)] = pvcUID
		return nil
	})
}

func (r *MariaDBReconciler) clearReplicaRecoveryCompletedPVCUIDAnnotation(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB, podIndex int) error {
	key := client.ObjectKeyFromObject(mariadb)
	annotationKey := replicaRecoveryCompletedPVCUIDAnnotationKey(podIndex)

	var current mariadbv1alpha1.MariaDB
	if err := r.Get(ctx, key, &current); err != nil {
		return fmt.Errorf("error getting MariaDB: %v", err)
	}
	if current.Annotations == nil || current.Annotations[annotationKey] == "" {
		return nil
	}

	return r.patch(ctx, &current, func(mdb *mariadbv1alpha1.MariaDB) error {
		delete(mdb.Annotations, annotationKey)
		return nil
	})
}

func (r *MariaDBReconciler) clearReplicaRecoveryCompletedPVCUIDAnnotations(ctx context.Context,
	mariadb *mariadbv1alpha1.MariaDB) error {
	key := client.ObjectKeyFromObject(mariadb)

	var current mariadbv1alpha1.MariaDB
	if err := r.Get(ctx, key, &current); err != nil {
		return fmt.Errorf("error getting MariaDB: %v", err)
	}
	if len(replicaRecoveryCompletedPVCUIDTrackedAnnotations(current.Annotations)) == 0 {
		return nil
	}

	return r.patch(ctx, &current, func(mdb *mariadbv1alpha1.MariaDB) error {
		for key := range mdb.Annotations {
			if strings.HasPrefix(key, replicaRecoveryCompletedPVCUIDAnnotationPrefix) {
				delete(mdb.Annotations, key)
			}
		}
		return nil
	})
}

func (r *MariaDBReconciler) refreshManagedSQLResources(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, token string) error {
	if err := r.refreshManagedDatabases(ctx, mariadb, token); err != nil {
		return err
	}
	if err := r.refreshManagedUsers(ctx, mariadb, token); err != nil {
		return err
	}
	if err := r.refreshManagedGrants(ctx, mariadb, token); err != nil {
		return err
	}
	return nil
}

func (r *MariaDBReconciler) refreshManagedDatabases(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, token string) error {
	var databaseList mariadbv1alpha1.DatabaseList
	if err := r.List(ctx, &databaseList, client.InNamespace(mariadb.Namespace)); err != nil {
		return fmt.Errorf("error listing Databases: %v", err)
	}

	for i := range databaseList.Items {
		database := &databaseList.Items[i]
		if !referencesMariaDB(database.Spec.MariaDBRef, mariadb) {
			continue
		}
		if err := patchSQLReconcileToken(ctx, r.Client, database, token); err != nil {
			return fmt.Errorf("error refreshing Database '%s': %v", database.Name, err)
		}
	}
	return nil
}

func (r *MariaDBReconciler) refreshManagedUsers(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, token string) error {
	var userList mariadbv1alpha1.UserList
	if err := r.List(ctx, &userList, client.InNamespace(mariadb.Namespace)); err != nil {
		return fmt.Errorf("error listing Users: %v", err)
	}

	for i := range userList.Items {
		user := &userList.Items[i]
		if !referencesMariaDB(user.Spec.MariaDBRef, mariadb) {
			continue
		}
		if err := patchSQLReconcileToken(ctx, r.Client, user, token); err != nil {
			return fmt.Errorf("error refreshing User '%s': %v", user.Name, err)
		}
	}
	return nil
}

func (r *MariaDBReconciler) refreshManagedGrants(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, token string) error {
	var grantList mariadbv1alpha1.GrantList
	if err := r.List(ctx, &grantList, client.InNamespace(mariadb.Namespace)); err != nil {
		return fmt.Errorf("error listing Grants: %v", err)
	}

	for i := range grantList.Items {
		grant := &grantList.Items[i]
		if !referencesMariaDB(grant.Spec.MariaDBRef, mariadb) {
			continue
		}
		if err := patchSQLReconcileToken(ctx, r.Client, grant, token); err != nil {
			return fmt.Errorf("error refreshing Grant '%s': %v", grant.Name, err)
		}
	}
	return nil
}

func referencesMariaDB(ref mariadbv1alpha1.MariaDBRef, mariadb *mariadbv1alpha1.MariaDB) bool {
	return ref.Name == mariadb.Name && (ref.Namespace == "" || ref.Namespace == mariadb.Namespace)
}

func patchSQLReconcileToken(ctx context.Context, c client.Client, obj client.Object, token string) error {
	annotations := obj.GetAnnotations()
	if annotations != nil && annotations[sqlReconcileTokenAnnotation] == token {
		return nil
	}

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	newAnnotations := make(map[string]string, len(annotations)+1)
	for key, value := range annotations {
		newAnnotations[key] = value
	}
	newAnnotations[sqlReconcileTokenAnnotation] = token
	obj.SetAnnotations(newAnnotations)

	return c.Patch(ctx, obj, patch)
}

func (r *MariaDBReconciler) syncStoragePVCUIDAnnotation(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB,
	podIndex int) error {
	var pvc corev1.PersistentVolumeClaim
	pvcKey := mariadb.PVCKey(builder.StorageVolume, podIndex)
	if err := r.Get(ctx, pvcKey, &pvc); err != nil {
		return fmt.Errorf("error getting PVC '%s': %v", pvcKey.Name, err)
	}

	var current mariadbv1alpha1.MariaDB
	key := client.ObjectKeyFromObject(mariadb)
	if err := r.Get(ctx, key, &current); err != nil {
		return fmt.Errorf("error getting MariaDB: %v", err)
	}

	annotationKey := storagePVCUIDAnnotationKey(podIndex)
	if current.Annotations != nil && current.Annotations[annotationKey] == string(pvc.UID) {
		return nil
	}

	return r.patch(ctx, &current, func(mdb *mariadbv1alpha1.MariaDB) error {
		if mdb.Annotations == nil {
			mdb.Annotations = map[string]string{}
		}
		mdb.Annotations[annotationKey] = string(pvc.UID)
		return nil
	})
}

func (r *MariaDBReconciler) ensureStoragePVCPresent(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, podIndex int,
	logger logr.Logger) (ctrl.Result, error) {
	var pvc corev1.PersistentVolumeClaim
	pvcKey := mariadb.PVCKey(builder.StorageVolume, podIndex)
	if err := r.Get(ctx, pvcKey, &pvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error getting PVC '%s': %v", pvcKey.Name, err)
		}

		logger.Info("Recreating missing storage PVC", "pvc", pvcKey.Name)
		if err := r.reconcilePVC(ctx, mariadb, pvcKey); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reconciling PVC '%s': %v", pvcKey.Name, err)
		}
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	if pvc.Status.Phase != corev1.ClaimBound {
		logger.V(1).Info("Waiting for storage PVC to be bound", "pvc", pvc.Name, "phase", pvc.Status.Phase)
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}
