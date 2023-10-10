package conditions

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/statefulset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetPrimarySwitching(c Conditioner, mariadb *mariadbv1alpha1.MariaDB) {
	msg := switchingPrimaryMessage(mariadb)
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonSwitchPrimary,
		Message: msg,
	})
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypePrimarySwitched,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonSwitchPrimary,
		Message: msg,
	})
}

func SetPrimarySwitched(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypePrimarySwitched,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonSwitchPrimary,
		Message: "Switchover complete",
	})
}

func switchingPrimaryMessage(mariadb *mariadbv1alpha1.MariaDB) string {
	return fmt.Sprintf(
		"Switching primary to '%s'",
		statefulset.PodName(mariadb.ObjectMeta, *mariadb.Replication().Primary.PodIndex),
	)
}
