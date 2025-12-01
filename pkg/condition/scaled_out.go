package conditions

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetScaledOut(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeScaledOut,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonScaledOut,
		Message: "Scaled out",
	})
}

func SetScalingOut(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeScaledOut,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonScalingOut,
		Message: "Scaling out",
	})
}

func SetScaleOutError(c Conditioner, msg string) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeScaledOut,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonScaleOutError,
		Message: fmt.Sprintf("Scale out error: %s", msg),
	})
}
