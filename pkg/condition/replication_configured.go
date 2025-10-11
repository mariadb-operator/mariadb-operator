package conditions

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetReplicationConfigured(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReplicationConfigured,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonReplicationConfigured,
		Message: "Replication configured",
	})
}
