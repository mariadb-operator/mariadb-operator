package conditions

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetReadyHealthty(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonHealthy,
		Message: "Healthy",
	})
}

func SetReadyUnhealthtyWithError(c Conditioner, err error) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonHealthy,
		Message: err.Error(),
	})
}

func SetReadyCreatedWithMessage(c Conditioner, message string) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonCreated,
		Message: message,
	})
}

func SetReadyCreated(c Conditioner) {
	SetReadyCreatedWithMessage(c, "Created")
}

func SetReadyFailedWithMessage(c Conditioner, message string) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonFailed,
		Message: message,
	})
}

func SetReadyFailed(c Conditioner) {
	SetReadyFailedWithMessage(c, "Failed")
}

func SetReadyWithStatefulSet(c Conditioner, sts *appsv1.StatefulSet) {
	if sts.Status.Replicas == 0 || sts.Status.ReadyReplicas != sts.Status.Replicas {
		c.SetCondition(metav1.Condition{
			Type:    mariadbv1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  mariadbv1alpha1.ConditionReasonStatefulSetNotReady,
			Message: "Not ready",
		})
		return
	}
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonStatefulSetReady,
		Message: "Running",
	})
}
