package conditions

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetRecoveringGalera(c Conditioner, mariadb *mariadbv1alpha1.MariaDB) {
	msg := switchingPrimaryMessage(mariadb)
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonRecoverGalera,
		Message: msg,
	})
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeGaleraRecovered,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonRecoverGalera,
		Message: "Recovering Galera cluster",
	})
}

func SetGaleraRecovered(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypePrimarySwitched,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonSwitchPrimary,
		Message: "Recovered Galera cluster",
	})
}
