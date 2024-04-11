package conditions

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetGaleraBootstrapping(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonGaleraNotReady,
		Message: "Bootstrapping Galera",
	})
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeGaleraReady,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonGaleraNotReady,
		Message: "Bootstrapping Galera",
	})
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeGaleraBootstrapped,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionTypeGaleraBootstrapped,
		Message: "Bootstrapping Galera",
	})
}

func SetGaleraBootstrapped(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeGaleraBootstrapped,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionTypeGaleraBootstrapped,
		Message: "Galera bootstrapped",
	})
}
