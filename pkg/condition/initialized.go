package conditions

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetInitialized(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeInitialized,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonInitialized,
		Message: "Initialized",
	})
}

func SetInitializing(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeInitialized,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonInitializing,
		Message: "Initializing",
	})
}

func SetInitError(c Conditioner, msg string) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeInitialized,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonInitError,
		Message: fmt.Sprintf("Init error: %s", msg),
	})
}
