package pod

import (
	"context"
	"errors"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	labels "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/labels"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/statefulset"
	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func PodReadyCondition(pod *corev1.Pod) *corev1.PodCondition {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return &c
		}
	}
	return nil
}

func PodReady(pod *corev1.Pod) bool {
	if c := PodReadyCondition(pod); c != nil {
		return c.Status == corev1.ConditionTrue
	}
	return false
}

func PodUpdated(pod *corev1.Pod, updateRevision string) bool {
	if podUpdateRevision, ok := pod.Labels["controller-revision-hash"]; ok {
		return podUpdateRevision == updateRevision
	}
	return false
}

func PodScheduled(pod *corev1.Pod) bool {
	return pod.Spec.NodeName != ""
}

func PodInitializing(pod *corev1.Pod) bool {
	for _, ics := range pod.Status.InitContainerStatuses {
		if ics.State.Running != nil {
			return true
		}
	}
	return false
}

func ListMariaDBPods(ctx context.Context, client ctrlclient.Client,
	mariadb *mariadbv1alpha1.MariaDB) ([]corev1.Pod, error) {
	var podList corev1.PodList
	if err := client.List(
		ctx,
		&podList,
		ctrlclient.InNamespace(mariadb.Namespace),
		ctrlclient.MatchingLabels(
			labels.NewLabelsBuilder().
				WithMariaDBSelectorLabels(mariadb).
				Build(),
		),
	); err != nil {
		return nil, err
	}
	pods := make([]corev1.Pod, 0, len(podList.Items))
	for _, p := range podList.Items {
		// ignore Pods created by Jobs
		if IsManagedByJob(p) {
			continue
		}
		pods = append(pods, p)
	}
	return pods, nil
}

func ListMariaDBSecondaryPods(ctx context.Context, client ctrlclient.Client,
	mariadb *mariadbv1alpha1.MariaDB) ([]corev1.Pod, error) {
	if mariadb.Status.CurrentPrimaryPodIndex == nil {
		return nil, errors.New("'status.currentPrimaryPodIndex' must be set")
	}
	pods, err := ListMariaDBPods(ctx, client, mariadb)
	if err != nil {
		return nil, err
	}
	secondaryPods := make([]corev1.Pod, 0, len(pods))
	for _, p := range pods {
		podIndex, err := statefulset.PodIndex(p.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting Pod '%s' index: %v", p.Name, err)
		}
		if *podIndex == *mariadb.Status.CurrentPrimaryPodIndex {
			continue
		}
		secondaryPods = append(secondaryPods, p)
	}
	return secondaryPods, nil
}

func IsManagedByJob(pod corev1.Pod) bool {
	return pod.Labels["job-name"] != "" ||
		pod.Labels["batch.kubernetes.io/job-name"] != ""
}
