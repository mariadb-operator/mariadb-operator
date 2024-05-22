package conditions

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetPendingUpdate(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeUpdated,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonPendingUpdate,
		Message: "Pending update",
	})
}

func SetUpdating(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeUpdated,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonUpdating,
		Message: "Updating",
	})
}

func SetUpdated(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeUpdated,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonUpdated,
		Message: "Updated",
	})
}
