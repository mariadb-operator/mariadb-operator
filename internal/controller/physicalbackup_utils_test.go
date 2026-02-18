package controller

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/job"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/volumesnapshot"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

// Holds utility functions shared between Replication and PhysicalBackup tests.

func testPhysicalBackup(backup *mariadbv1alpha1.PhysicalBackup) {
	By("Creating PhysicalBackup")
	Expect(k8sClient.Create(testCtx, backup)).To(Succeed())
	DeferCleanup(func() {
		Expect(client.IgnoreNotFound(k8sClient.Delete(testCtx, backup))).To(Succeed())
	})

	if backup.Spec.Storage.VolumeSnapshot != nil {
		testPhysicalBackupVolumeSnapshot(backup)
	} else {
		testPhysicalBackupJob(backup)
	}
}

func testPhysicalBackupJob(backup *mariadbv1alpha1.PhysicalBackup) {
	key := client.ObjectKeyFromObject(backup)

	By("Expecting to create a ServiceAccount eventually")
	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(testCtx, key, backup)).To(Succeed())
		var svcAcc corev1.ServiceAccount
		key := backup.ServiceAccountKey()
		g.Expect(k8sClient.Get(testCtx, key, &svcAcc)).To(Succeed())
		return true
	}, testTimeout, testInterval).Should(BeTrue())

	var jobList *batchv1.JobList
	By("Expecting to create a Job eventually")
	Eventually(func() bool {
		var err error
		jobList, err = job.ListJobs(testCtx, k8sClient, backup)
		if err != nil {
			return false
		}
		return len(jobList.Items) > 0
	}, testTimeout, testInterval).Should(BeTrue())

	job := jobList.Items[0]
	By("Expecting Job to have mariadb init container")
	Expect(job.Spec.Template.Spec.InitContainers).To(ContainElement(MatchFields(IgnoreExtras,
		Fields{
			"Name": Equal("mariadb"),
		})))

	By("Expecting Job to have mariadb-operator container")
	Expect(job.Spec.Template.Spec.Containers).To(ContainElement(MatchFields(IgnoreExtras,
		Fields{
			"Name": Equal("mariadb-operator"),
		})))

	By("Expecting PhysicalBackup to complete eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, backup); err != nil {
			return false
		}
		if backup.IsComplete() {
			return true
		}

		var podList corev1.PodList
		if err := k8sClient.List(testCtx, &podList, client.InNamespace(backup.Namespace)); err == nil {
			for _, p := range podList.Items {
				fmt.Fprintf(GinkgoWriter, "Pod: %s, Status: %s\n", p.Name, p.Status.Phase)
				for _, cs := range p.Status.ContainerStatuses {
					fmt.Fprintf(
						GinkgoWriter,
						"	Container: %s, Ready: %t, RestartCount: %d, State: %v, LastTerminationState: %v\n",
						cs.Name,
						cs.Ready,
						cs.RestartCount,
						cs.State,
						cs.LastTerminationState,
					)
				}
			}
		}
		return false
	}, testTimeout, testInterval).Should(BeTrue())
}

func testPhysicalBackupVolumeSnapshot(backup *mariadbv1alpha1.PhysicalBackup) {
	key := client.ObjectKeyFromObject(backup)

	By("Expecting to create a VolumeSnapshot eventually")
	Eventually(func() bool {
		volumeSnapshotList, err := volumesnapshot.ListVolumeSnapshots(testCtx, k8sClient, backup)
		if err != nil {
			return false
		}
		return len(volumeSnapshotList.Items) > 0
	}, testTimeout, testInterval).Should(BeTrue())

	By("Expecting PhysicalBackup to complete eventually")
	Eventually(func() bool {
		if err := k8sClient.Get(testCtx, key, backup); err != nil {
			return false
		}
		if backup.IsComplete() {
			return true
		}

		var podList corev1.PodList
		if err := k8sClient.List(testCtx, &podList, client.InNamespace(backup.Namespace)); err == nil {
			for _, p := range podList.Items {
				fmt.Fprintf(GinkgoWriter, "Pod: %s, Status: %s\n", p.Name, p.Status.Phase)
				for _, cs := range p.Status.ContainerStatuses {
					fmt.Fprintf(
						GinkgoWriter,
						"	Container: %s, Ready: %t, RestartCount: %d, State: %v, LastTerminationState: %v\n",
						cs.Name,
						cs.Ready,
						cs.RestartCount,
						cs.State,
						cs.LastTerminationState,
					)
				}
			}
		}
		return false
	}, testTimeout, testInterval).Should(BeTrue())
}
