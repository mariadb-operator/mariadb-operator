package pod

import (
	corev1 "k8s.io/api/core/v1"
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
	if podUpdateRevision, ok := pod.ObjectMeta.Labels["controller-revision-hash"]; ok {
		return podUpdateRevision == updateRevision
	}
	return false
}
