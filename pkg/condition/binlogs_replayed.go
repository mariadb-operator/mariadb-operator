package conditions

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetReplayingBinlogs(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonReplayBinlogs,
		Message: "Replaying binlogs",
	})
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeBinlogsReplayed,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonReplayBinlogs,
		Message: "Replaying binlogs",
	})
}

func SetReplayedBinlogs(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeBinlogsReplayed,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonReplayBinlogs,
		Message: "Replayed binlogs",
	})
}

func SetReplayBinlogsError(c Conditioner, msg string) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeBinlogsReplayed,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonReplayBinlogsError,
		Message: fmt.Sprintf("Error replaying binlogs: %s", msg),
	})
}
