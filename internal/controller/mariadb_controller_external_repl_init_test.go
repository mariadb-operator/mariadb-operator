package controller

import (
	"fmt"
	"strconv"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/refresolver"
	sqlClient "github.com/mariadb-operator/mariadb-operator/v26/pkg/sql"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MariaDB replication from external server", Ordered, func() {

	var (
		key           = testMdbERkey
		pbRecoveryKey = testMdbPbRecoveryERkey
		mdb           = &mariadbv1alpha1.MariaDB{}
	)

	It("should reconcile", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, testMdbERkey, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a Service")
		var svc corev1.Service
		Expect(k8sClient.Get(testCtx, key, &svc)).To(Succeed())

		By("Expecting to create a primary Service")
		Expect(k8sClient.Get(testCtx, mdb.PrimaryServiceKey(), &svc)).To(Succeed())
		Expect(svc.Spec.Selector["statefulset.kubernetes.io/pod-name"]).To(Equal(statefulset.PodName(mdb.ObjectMeta, 0)))

		By("Expecting to create a secondary Service")
		Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &svc)).To(Succeed())

		By("Expecting role label to be set to primary")
		Eventually(func() bool {
			currentPrimary := *mdb.Status.CurrentPrimary
			primaryPodKey := types.NamespacedName{
				Name:      currentPrimary,
				Namespace: mdb.Namespace,
			}
			var primaryPod corev1.Pod
			if err := k8sClient.Get(testCtx, primaryPodKey, &primaryPod); err != nil {
				return apierrors.IsNotFound(err)
			}
			return primaryPod.Labels["k8s.mariadb.com/role"] == "primary"
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting Connection to be ready eventually")
		Eventually(func() bool {
			var conn mariadbv1alpha1.Connection
			if err := k8sClient.Get(testCtx, key, &conn); err != nil {
				return false
			}
			return conn.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting primary Connection to be ready eventually")
		Eventually(func() bool {
			var conn mariadbv1alpha1.Connection
			if err := k8sClient.Get(testCtx, mdb.PrimaryConnectioneKey(), &conn); err != nil {
				return false
			}
			return conn.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting secondary Connection to be ready eventually")
		Eventually(func() bool {
			var conn mariadbv1alpha1.Connection
			if err := k8sClient.Get(testCtx, mdb.SecondaryConnectioneKey(), &conn); err != nil {
				return false
			}
			return conn.IsReady()
		}, testTimeout, testInterval).Should(BeTrue())
		var endpoints discoveryv1.EndpointSlice

		By("Expecting to create secondary Endpoints: " + strconv.Itoa(int(mdb.Spec.Replicas)))
		Eventually(func() bool {
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpoints)).To(Succeed())
			count := 0
			for _, address := range endpoints.Endpoints {
				if *address.Conditions.Ready {
					count++
				}
			}
			return count == int(mdb.Spec.Replicas)

		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting to create a PodDisruptionBudget")
		var pdb policyv1.PodDisruptionBudget
		Expect(k8sClient.Get(testCtx, key, &pdb)).To(Succeed())

		By("Expecting the logical backup to inherit resources from the template")
		refResolver := refresolver.New(k8sClient)
		emdb, err := refResolver.ExternalMariaDB(testCtx, &mdb.Replication().ReplicaFromExternal.MariaDBRef.ObjectReference, testNamespace)
		Expect(err).To(Succeed())
		var logicalBackup mariadbv1alpha1.Backup
		Expect(k8sClient.Get(testCtx, types.NamespacedName{
			Name:      mdb.ExternalReplLogicalBackupName(),
			Namespace: emdb.Namespace,
		}, &logicalBackup)).To(Succeed())
		Expect(logicalBackup.Spec.Resources).NotTo(BeNil())
		Expect(logicalBackup.Spec.Resources.Limits.Cpu().String()).To(Equal("300m"))
		Expect(logicalBackup.Spec.Resources.Limits.Memory().String()).To(Equal("512Mi"))
		Expect(logicalBackup.Spec.Resources.Requests.Cpu().String()).To(Equal("100m"))
		Expect(logicalBackup.Spec.Resources.Requests.Memory().String()).To(Equal("128Mi"))

		By("Expecting each Restore to inherit resources from replica.bootstrapFrom.restoreJob")
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			var restore mariadbv1alpha1.Restore
			err := k8sClient.Get(testCtx, mdb.RestoreKeyInPod(i), &restore)
			if apierrors.IsNotFound(err) {
				continue
			}
			Expect(err).To(Succeed())
			Expect(restore.Spec.Resources).NotTo(BeNil())
			Expect(restore.Spec.Resources.Limits.Cpu().String()).To(Equal("300m"))
			Expect(restore.Spec.Resources.Limits.Memory().String()).To(Equal("512Mi"))
			Expect(restore.Spec.Resources.Requests.Cpu().String()).To(Equal("100m"))
			Expect(restore.Spec.Resources.Requests.Memory().String()).To(Equal("128Mi"))
		}
	})

	It("should recover if replication is broken", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady() && mdb.IsExternalReplInitialized()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting to get SqlClient from Pod 2")
		refResolver := refresolver.New(k8sClient)
		var client *sqlClient.Client
		var err error
		podIndex := 2
		client, err = sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver, podIndex)
		Expect(err).To(Succeed())
		defer client.Close()

		By("Expecting to break replication on Pod 2")
		Expect(
			client.Exec(testCtx, "STOP SLAVE;"),
			client.Exec(testCtx, "RESET MASTER;"),
			client.Exec(testCtx, "RESET SLAVE;"),
			client.Exec(testCtx, "SET GLOBAL gtid_slave_pos='0-1-0';"),
			client.Exec(testCtx, "START SLAVE;"),
		).To(Succeed())

		By("Expecting replication to be ready eventually on Pod " + strconv.Itoa(podIndex))
		Eventually(func() bool {
			isReplicaHealthy, _ := client.IsReplicationHealthy(testCtx)
			return isReplicaHealthy
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting replication status to get back to slave Pod " + strconv.Itoa(podIndex))
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return apierrors.IsNotFound(err)
			}
			return (mdb.Status.Replication.Roles)[statefulset.PodName(mdb.ObjectMeta, podIndex)] == mariadbv1alpha1.ReplicationRoleReplica
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB status to get back to running and Ready")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return apierrors.IsNotFound(err)
			}
			condition := meta.FindStatusCondition(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReady)
			return condition != nil && condition.Status == metav1.ConditionTrue
		}, testHighTimeout, testInterval).Should(BeTrue())

		var endpoints discoveryv1.EndpointSlice
		By("Expecting Pod " + strconv.Itoa(podIndex) + " to present on the secondary endpoints")
		Eventually(func() bool {
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpoints)).To(Succeed())

			podKey := types.NamespacedName{
				Name:      statefulset.PodName(mdb.ObjectMeta, podIndex),
				Namespace: testNamespace,
			}
			var pod corev1.Pod
			Expect(k8sClient.Get(testCtx, podKey, &pod)).To(Succeed())

			for _, address := range endpoints.Endpoints {
				if address.Addresses[0] == pod.Status.PodIP && *address.Conditions.Ready {
					return true
				}
			}
			return false
		}, testTimeout, testInterval).Should(BeTrue())
	})

	It("should recover in case of missing GTID replication error (1236)", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting to get SqlClient from Pod 2")
		refResolver := refresolver.New(k8sClient)
		var client *sqlClient.Client
		var err error
		podIndex := 2
		client, err = sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver, podIndex)
		Expect(err).To(Succeed())
		defer client.Close()

		By("Suspend MariaDB")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			mdb.Spec.Suspend = true

			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to eventually be suspended")
		expectMariadbFn(testCtx, k8sClient, key, func(mdb *mariadbv1alpha1.MariaDB) bool {
			condition := meta.FindStatusCondition(mdb.Status.Conditions, mariadbv1alpha1.ConditionTypeReady)
			if condition == nil {
				return false
			}
			return condition.Status == metav1.ConditionFalse && condition.Reason == mariadbv1alpha1.ConditionReasonSuspended
		})

		By("Expecting to stop replication on Pod 2")
		Expect(client.Exec(testCtx, "STOP SLAVE")).To(Succeed(), client.Exec(testCtx, "SET GLOBAL gtid_slave_pos = '0-9999-9999'"))

		By("Expecting to set Invalid GTID position on Pod 2")
		Expect(client.Exec(testCtx, "SET GLOBAL gtid_slave_pos = '0-9999-9999'")).To(Succeed())

		By("Expecting to start replication on Pod 2")
		Expect(client.Exec(testCtx, "START SLAVE")).To(Succeed())

		By("Expecting replication error 1236 on Pod " + strconv.Itoa(podIndex))
		Eventually(func() bool {
			rStatus, err := client.GetReplicationStatus(testCtx)
			if err != nil {
				return false
			}
			return rStatus.LastIOErrno.Int32 == 1236

		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Resume MariaDB")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			mdb.Spec.Suspend = false

			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting no replication error 1236 on Pod " + strconv.Itoa(podIndex))
		Eventually(func() bool {
			rStatus, err := client.GetReplicationStatus(testCtx)
			if err != nil {
				return false
			}
			return rStatus.LastIOErrno.Int32 != 1236

		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Expecting replication status to get back to slave Pod " + strconv.Itoa(podIndex))
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return apierrors.IsNotFound(err)
			}
			return (mdb.Status.Replication.Roles)[statefulset.PodName(mdb.ObjectMeta, podIndex)] == mariadbv1alpha1.ReplicationRoleReplica
		}, testHighTimeout, testInterval).Should(BeTrue())

		var endpoints discoveryv1.EndpointSlice
		By("Expecting Pod " + strconv.Itoa(podIndex) + " to present on the secondary endpoints")
		Eventually(func() bool {
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpoints)).To(Succeed())

			podKey := types.NamespacedName{
				Name:      statefulset.PodName(mdb.ObjectMeta, podIndex),
				Namespace: testNamespace,
			}
			var pod corev1.Pod
			Expect(k8sClient.Get(testCtx, podKey, &pod)).To(Succeed())

			for _, address := range endpoints.Endpoints {
				if address.Addresses[0] == pod.Status.PodIP && *address.Conditions.Ready {
					return true
				}
			}
			return false
		}, testTimeout, testInterval).Should(BeTrue())

	})

	It("should reuse physical backup if it still valid", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		// Get current backup age
		By("Expecting to get current external MariadDB object")
		refResolver := refresolver.New(k8sClient)
		emdb, err := refResolver.ExternalMariaDB(testCtx, &mdb.Replication().ReplicaFromExternal.MariaDBRef.ObjectReference, testNamespace)
		Expect(err).To(Succeed())

		emdbKey := types.NamespacedName{
			Name:      emdb.Name,
			Namespace: emdb.Namespace,
		}
		var existingLogicalBackup mariadbv1alpha1.Backup
		By("Expecting to get backup object")
		err = k8sClient.Get(testCtx, emdbKey, &existingLogicalBackup)
		Expect(err).To(Succeed())
		firstLogicalBackupCreationTimestamp := existingLogicalBackup.CreationTimestamp.Time

		var existingPhysicalBackup mariadbv1alpha1.PhysicalBackup
		By("Expecting to get backup object")
		err = k8sClient.Get(testCtx, pbRecoveryKey, &existingPhysicalBackup)
		Expect(err).To(Succeed())
		firstPhysicalBackupCreationTimestamp := existingPhysicalBackup.CreationTimestamp.Time

		// Insert data on the external master to be sure that replica is not up to date with the master
		By("Expecting to get SqlClient from the external MariaDB")
		client, err := sqlClient.NewClientWithMariaDB(testCtx, emdb, refResolver)
		Expect(err).To(Succeed())
		defer client.Close()

		By("Expecting to insert data on the external master")
		Expect(
			client.Exec(testCtx, "CREATE DATABASE IF NOT EXISTS test;"),
			client.Exec(testCtx, "USE test;"),
			client.Exec(testCtx, "CREATE TABLE IF NOT EXISTS t (id INT PRIMARY KEY);"),
			client.Exec(testCtx, "INSERT INTO t VALUES (1);"),
		).To(Succeed())

		testDeletePod(mdb, 2, true)

		// Expect to get in recovering state eventually
		By("Expecting MariaDB to be in recovering state eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}

			return mdb.IsRecoveringReplicas()

		}, testHighTimeout, testInterval).Should(BeTrue())

		// Expect to get back to ready state eventually
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()

			// return (mdb.Status.Replication.Roles)[statefulset.PodName(mdb.ObjectMeta, podIndex)] == mariadbv1alpha1.ReplicationRoleReplica
		}, testHighTimeout, testInterval).Should(BeTrue())

		// Get current Physical backup age
		By("Expecting to get physical backup object")
		err = k8sClient.Get(testCtx, pbRecoveryKey, &existingPhysicalBackup)
		Expect(err).To(Succeed())
		secondPhysicalBackupCreationTimestamp := existingPhysicalBackup.CreationTimestamp.Time

		// Physical backup should not be updated as it's still valid
		By("Expecting to have same CreationTimestamp on physical backup Object before and after the Pod recreation")
		Expect(firstPhysicalBackupCreationTimestamp).To(Equal(secondPhysicalBackupCreationTimestamp))

		// Get current backup age
		By("Expecting to get backup object")
		err = k8sClient.Get(testCtx, emdbKey, &existingLogicalBackup)
		Expect(err).To(Succeed())
		secondLogicalBackupCreationTimestamp := existingLogicalBackup.CreationTimestamp.Time

		// Last age should be older than first
		By("Expecting to have same CreationTimestamp on backup Object before and after the Pod recreation")
		Expect(firstLogicalBackupCreationTimestamp).To(Equal(secondLogicalBackupCreationTimestamp))

	})

	It("should invalidate physical backup if older than the master binlog retention period", func() {
		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		// Get current backup age
		By("Expecting to get current external MariadDB object")
		refResolver := refresolver.New(k8sClient)
		emdb, err := refResolver.ExternalMariaDB(testCtx, &mdb.Replication().ReplicaFromExternal.MariaDBRef.ObjectReference, testNamespace)
		Expect(err).To(Succeed())

		logicalBackupKey := types.NamespacedName{
			Name:      emdb.Name,
			Namespace: emdb.Namespace,
		}

		var existingLogicalBackup mariadbv1alpha1.Backup
		By("Expecting to get backup object")
		err = k8sClient.Get(testCtx, logicalBackupKey, &existingLogicalBackup)
		Expect(err).To(Succeed())
		firstLogicalBackupCreationTimestamp := existingLogicalBackup.CreationTimestamp.Time

		var existingPhysicalBackup mariadbv1alpha1.PhysicalBackup
		By("Expecting to get backup object")
		err = k8sClient.Get(testCtx, pbRecoveryKey, &existingPhysicalBackup)
		Expect(err).To(Succeed())
		firstPhysicalBackupCreationTimestamp := existingPhysicalBackup.CreationTimestamp.Time

		podIndex := 2

		// Change binlog_expire_logs_seconds to 10 on the master server
		By("Expecting to get SqlClient from the external MariaDB")
		client, err := sqlClient.NewClientWithMariaDB(testCtx, emdb, refResolver)
		Expect(err).To(Succeed())
		defer client.Close()

		By("Expecting to set binlog_expire_logs_seconds to 10 on the master server")
		Expect(client.SetSystemVariable(testCtx, "binlog_expire_logs_seconds", "10")).To(Succeed())

		testDeletePod(mdb, podIndex, true)

		By("Expecting MariaDB to be in recovering state eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}

			return mdb.IsRecoveringReplicas()

		}, testHighTimeout, testInterval).Should(BeTrue())

		// Revert binlog_expire_logs_seconds to 30 days on the master server
		By("Expecting to set expire_logs_days to 30 on the master server")
		Expect(client.SetSystemVariable(testCtx, "expire_logs_days", "30")).To(Succeed())

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()

			// return (mdb.Status.Replication.Roles)[statefulset.PodName(mdb.ObjectMeta, podIndex)] == mariadbv1alpha1.ReplicationRoleReplica
		}, testHighTimeout, testInterval).Should(BeTrue())

		// Get current Physical backup age
		By("Expecting to get physical backup object")
		err = k8sClient.Get(testCtx, pbRecoveryKey, &existingPhysicalBackup)
		Expect(err).To(Succeed())
		secondPhysicalBackupCreationTimestamp := existingPhysicalBackup.CreationTimestamp.Time

		// Physical backup should be updated as it's older than the master binlog retention period
		By("Expecting to have different CreationTimestamp on physical backup Object before and after the Pod recreation")
		Expect(firstPhysicalBackupCreationTimestamp).ShouldNot(Equal(secondPhysicalBackupCreationTimestamp))

		// Get current backup age
		By("Expecting to get backup object")
		err = k8sClient.Get(testCtx, logicalBackupKey, &existingLogicalBackup)
		Expect(err).To(Succeed())
		secondLogicalBackupCreationTimestamp := existingLogicalBackup.CreationTimestamp.Time

		// Logical backup should should not be touched as the cluster still has valid replicas for a physical backup
		By("Expecting to have same CreationTimestamp on backup Object before and after the Pod recreation")
		Expect(firstLogicalBackupCreationTimestamp).Should(Equal(secondLogicalBackupCreationTimestamp))

		// Revert binlog_expire_logs_seconds to 30 days on the master server
		By("Expecting to set expire_logs_days to 30 on the master server")
		Expect(client.SetSystemVariable(testCtx, "expire_logs_days", "30")).To(Succeed())
	})

	It("should invalidate logical backup if older than the master binlog retention period and no phy backup is avail and no valid replicas",
		func() {
			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, mdb); err != nil {
					return false
				}
				return mdb.IsReady()
			}, testHighTimeout, testInterval).Should(BeTrue())

			// Get current backup age
			By("Expecting to get current external MariadDB object")
			refResolver := refresolver.New(k8sClient)
			emdb, err := refResolver.ExternalMariaDB(testCtx, &mdb.Replication().ReplicaFromExternal.MariaDBRef.ObjectReference, testNamespace)
			Expect(err).To(Succeed())

			logicalBackupKey := types.NamespacedName{
				Name:      emdb.Name,
				Namespace: emdb.Namespace,
			}

			var existingLogicalBackup mariadbv1alpha1.Backup
			By("Expecting to get backup object")
			err = k8sClient.Get(testCtx, logicalBackupKey, &existingLogicalBackup)
			Expect(err).To(Succeed())
			firstLogicalBackupCreationTimestamp := existingLogicalBackup.CreationTimestamp.Time

			var existingPhysicalBackup mariadbv1alpha1.PhysicalBackup
			By("Expecting to get backup object")
			err = k8sClient.Get(testCtx, pbRecoveryKey, &existingPhysicalBackup)
			Expect(err).To(Succeed())
			firstPhysicalBackupCreationTimestamp := existingPhysicalBackup.CreationTimestamp.Time

			// Change binlog_expire_logs_seconds to 10 on the master server
			By("Expecting to get SqlClient from the external MariaDB")
			client, err := sqlClient.NewClientWithMariaDB(testCtx, emdb, refResolver)
			Expect(err).To(Succeed())
			defer client.Close()

			By("Expecting to set binlog_expire_logs_seconds to 30 on the master server")
			Expect(client.SetSystemVariable(testCtx, "binlog_expire_logs_seconds", "30")).To(Succeed())

			// Delete physical backup to be sure that only logical backup is available
			By("Expecting to delete physical backup")
			Expect(k8sClient.Delete(testCtx, &existingPhysicalBackup)).To(Succeed())

			// Delete all replicas to be sure that no valid replica exists for a physical backup
			for i := 0; i < int(mdb.Spec.Replicas); i++ {
				testDeletePod(mdb, i, true)
			}

			By("Expecting MariaDB to be in recovering state eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, mdb); err != nil {
					return false
				}
				return mdb.IsRecoveringReplicas()

			}, testHighTimeout, testInterval).Should(BeTrue())

			By("Expecting Logical backup to replaced eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, logicalBackupKey, &existingLogicalBackup); err != nil {
					return false
				}
				secondLogicalBackupCreationTimestamp := existingLogicalBackup.CreationTimestamp.Time
				return secondLogicalBackupCreationTimestamp.After(firstLogicalBackupCreationTimestamp)
			}, testHighTimeout, testInterval).Should(BeTrue())

			// Revert binlog_expire_logs_seconds to 30 days on the master server to avoid issues with other tests
			Expect(client.SetSystemVariable(testCtx, "expire_logs_days", "30")).To(Succeed())

			By("Expecting MariaDB to be ready eventually")
			Eventually(func() bool {
				if err := k8sClient.Get(testCtx, key, mdb); err != nil {
					return false
				}
				return mdb.IsReady()

				// return (mdb.Status.Replication.Roles)[statefulset.PodName(mdb.ObjectMeta, podIndex)] == mariadbv1alpha1.ReplicationRoleReplica
			}, testVeryHighTimeout, testInterval).Should(BeTrue())

			// Get current Physical backup age
			By("Expecting to get physical backup object")
			err = k8sClient.Get(testCtx, pbRecoveryKey, &existingPhysicalBackup)
			Expect(err).To(Succeed())
			secondPhysicalBackupCreationTimestamp := existingPhysicalBackup.CreationTimestamp.Time

			// Physical backup should be updated as it's older than the master binlog retention period
			By("Expecting to have different CreationTimestamp on physical backup Object before and after the Pod recreation")
			Expect(firstPhysicalBackupCreationTimestamp).ShouldNot(Equal(secondPhysicalBackupCreationTimestamp))

			// Get current backup age
			By("Expecting to get backup object")
			err = k8sClient.Get(testCtx, logicalBackupKey, &existingLogicalBackup)
			Expect(err).To(Succeed())
			secondLogicalBackupCreationTimestamp := existingLogicalBackup.CreationTimestamp.Time

			// Logical backup should be updated as it's older than the master binlog retention period and no physical backup
			// is available and no valid replicas exist
			By("Expecting to have different CreationTimestamp on backup Object before and after the Pod recreation")
			Expect(firstLogicalBackupCreationTimestamp).ShouldNot(Equal(secondLogicalBackupCreationTimestamp))

			// Revert binlog_expire_logs_seconds to 30 days on the master server
			By("Expecting to set expire_logs_days to 30 on the master server")
			Expect(client.SetSystemVariable(testCtx, "expire_logs_days", "30")).To(Succeed())
		})

	It("scale out replicas", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Increasing MariaDB replicas to 4")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			mdb.Spec.Replicas = 4

			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		var endpoints discoveryv1.EndpointSlice
		By("Expecting to create secondary Endpoints: 4")
		Eventually(func() bool {
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpoints)).To(Succeed())
			count := 0
			for _, address := range endpoints.Endpoints {
				if *address.Conditions.Ready {
					count++
				}
			}
			return count == 4
		}, testTimeout, testInterval).Should(BeTrue())

	})

	It("scale in replicas", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Decreasing MariaDB replicas to 3")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			mdb.Spec.Replicas = 3

			return k8sClient.Update(testCtx, mdb) == nil
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		var endpoints discoveryv1.EndpointSlice
		By("Expecting secondary Endpoints: 3")
		Eventually(func() bool {
			Expect(k8sClient.Get(testCtx, mdb.SecondaryServiceKey(), &endpoints)).To(Succeed())
			count := 0
			for _, address := range endpoints.Endpoints {
				if *address.Conditions.Ready {
					count++
				}
			}
			return count == 3
		}, testTimeout, testInterval).Should(BeTrue())

	})

	It("use the server_id offset", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady()
		}, testHighTimeout, testInterval).Should(BeTrue())

		offset := mdb.Replication().ReplicaFromExternal.ServerIdOffset
		replicas := int(mdb.Spec.Replicas)
		refResolver := refresolver.New(k8sClient)
		for i := 0; i < replicas; i++ {

			client, err := sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver, i)
			By("Expecting to get SqlClient from Pod " + strconv.Itoa(i))
			Expect(err).To(Succeed())

			server_id, err := client.SystemVariable(testCtx, "server_id")
			By("Expecting to get server_id from Pod " + strconv.Itoa(i))
			Expect(err).To(Succeed())

			server_id_int, _ := strconv.Atoi(server_id)

			By("Expecting server_id to be equal to podIndex + ServerIdOffset on Pod " + strconv.Itoa(i))
			Expect(server_id_int).To(Equal(i + *offset))

		}
	})

	It("should update", func() {
		By("Updating MariaDB")
		testMariadbUpdate(mdb)
	})

	It("should resize PVCs", func() {
		By("Resizing MariaDB PVCs")
		testMariadbVolumeResize(mdb, "400Mi")
	})

	It("should heal external master connection drift", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady() && mdb.IsExternalReplInitialized() && !mdb.IsRecoveringReplicas()
		}, testHighTimeout, testInterval).Should(BeTrue())

		By("Getting the desired external master host")
		var emdb mariadbv1alpha1.ExternalMariaDB
		Expect(k8sClient.Get(testCtx, testEMdbkey, &emdb)).To(Succeed())
		desiredHost := emdb.GetHost()
		Expect(desiredHost).NotTo(BeEmpty())

		// RFC 5737 TEST-NET-1 address, guaranteed not to be the real external master.
		const bogusHost = "192.0.2.123"

		By("Pointing every replica at a bogus master to simulate connection drift")
		refResolver := refresolver.New(k8sClient)
		for i := 0; i < int(mdb.Spec.Replicas); i++ {

			var podClient *sqlClient.Client
			Eventually(func() error {
				var err error
				podClient, err = sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver, i)
				return err
			}, testTimeout, testInterval).Should(Succeed())
			defer podClient.Close()

			Expect(podClient.StopAllSlaves(testCtx)).To(Succeed())
			Expect(podClient.Exec(testCtx, fmt.Sprintf("CHANGE MASTER TO MASTER_HOST='%s';", bogusHost))).To(Succeed())

			By(fmt.Sprintf("Verifying Pod %d master host has drifted", i))
			status, err := podClient.QueryColumnMap(testCtx, "SHOW REPLICA STATUS")
			Expect(err).To(Succeed())
			Expect(status["Master_Host"]).To(Equal(bogusHost))
		}

		By("Expecting the operator to re-point every replica at the external master and resume replication")
		refResolver2 := refresolver.New(k8sClient)
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			podClient, err := sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver2, i)
			Expect(err).To(Succeed())
			defer podClient.Close()

			Eventually(func(g Gomega) {
				status, err := podClient.QueryColumnMap(testCtx, "SHOW REPLICA STATUS")
				g.Expect(err).To(Succeed())
				g.Expect(status["Master_Host"]).To(Equal(desiredHost))
				g.Expect(status["Slave_IO_Running"]).To(Equal("Yes"))
				g.Expect(status["Slave_SQL_Running"]).To(Equal("Yes"))
			}, testHighTimeout, testInterval).Should(Succeed(),
				fmt.Sprintf("Pod %d should be re-pointed at the external master", i))
		}
	})

	It("should re-apply the replication password on an authentication error", func() {

		By("Expecting MariaDB to be ready eventually")
		Eventually(func() bool {
			if err := k8sClient.Get(testCtx, key, mdb); err != nil {
				return false
			}
			return mdb.IsReady() && mdb.IsExternalReplInitialized()
		}, testHighTimeout, testInterval).Should(BeTrue())

		// The master host and user are left untouched: only the password is broken. This exercises
		// the authentication-error repair path specifically, since no host/port/user drift exists.
		By("Breaking the replication credentials on every replica to trigger an authentication error")
		refResolver := refresolver.New(k8sClient)
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			podClient, err := sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver, i)
			Expect(err).To(Succeed())
			defer podClient.Close()

			Expect(podClient.StopAllSlaves(testCtx)).To(Succeed())
			Expect(podClient.Exec(testCtx, "CHANGE MASTER TO MASTER_PASSWORD='wrong-password';")).To(Succeed())
			Expect(podClient.StartSlave(testCtx)).To(Succeed())
		}

		By("Expecting the operator to re-apply the credentials and restore healthy replication")
		refResolver2 := refresolver.New(k8sClient)
		for i := 0; i < int(mdb.Spec.Replicas); i++ {
			podClient, err := sqlClient.NewInternalClientWithPodIndex(testCtx, mdb, refResolver2, i)
			Expect(err).To(Succeed())
			defer podClient.Close()

			Eventually(func(g Gomega) {
				status, err := podClient.QueryColumnMap(testCtx, "SHOW REPLICA STATUS")
				g.Expect(err).To(Succeed())
				g.Expect(status["Slave_IO_Running"]).To(Equal("Yes"))
				g.Expect(status["Slave_SQL_Running"]).To(Equal("Yes"))
			}, testHighTimeout, testInterval).Should(Succeed(),
				fmt.Sprintf("Pod %d should resume replication after credential repair", i))
		}
	})

})
