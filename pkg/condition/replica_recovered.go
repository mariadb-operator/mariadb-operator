package conditions

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetReplicaRecovered(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReplicaRecovered,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonReplicaRecovered,
		Message: "Replica recovered",
	})
}

func SetReplicaRecovering(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReplicaRecovered,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonReplicaRecovered,
		Message: "Recovering replica",
	})
}

func SetReplicaRecoveryError(c Conditioner, msg string) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReplicaRecovered,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonReplicaRecovered,
		Message: fmt.Sprintf("Replica recovery error: %s", msg),
	})
}
