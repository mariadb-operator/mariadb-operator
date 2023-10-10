package conditions

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetGaleraConfigured(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeGaleraConfigured,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonGaleraConfigured,
		Message: "Galera configured",
	})
}
