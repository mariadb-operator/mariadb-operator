package conditions

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetExternalReplInitialized(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeExternalReplInitialized,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonExternalReplInitialized,
		Message: "External replication initialized",
	})
}

func SetExternalReplInitializing(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeExternalReplInitialized,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonExternalReplInitialized,
		Message: "External replication initializing",
	})
}
