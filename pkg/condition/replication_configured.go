package conditions

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetConfiguringReplication(c Conditioner, mariadb *mariadbv1alpha1.MariaDB) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonConfigureReplication,
		Message: "Configuring replication",
	})
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReplicationConfigured,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonConfigureReplication,
		Message: "Configuring replication",
	})
}

func SetConfiguredReplication(c Conditioner, mariadb *mariadbv1alpha1.MariaDB) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReplicationConfigured,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonConfigureReplication,
		Message: "Configured replication",
	})
}
