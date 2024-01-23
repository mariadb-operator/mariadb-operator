package client

import (
	"encoding/json"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MonitorParameters struct {
	User                       string                                 `json:"user"`
	Password                   string                                 `json:"password"`
	MonitorInterval            metav1.Duration                        `json:"monitor_interval,omitempty"`
	CooperativeMonitoringLocks *mariadbv1alpha1.CooperativeMonitoring `json:"cooperative_monitoring_locks,omitempty"`
	Params                     MapParams                              `json:"-"`
}

func (m MonitorParameters) MarshalJSON() ([]byte, error) {
	type MonitorParametersInternal MonitorParameters // prevent recursion
	bytes, err := json.Marshal(MonitorParametersInternal(m))
	if err != nil {
		return nil, err
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(bytes, &rawMap); err != nil {
		return nil, err
	}

	for k, v := range m.Params {
		if _, ok := rawMap[k]; ok { // prevent overriding
			continue
		}
		bytes, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		rawMap[k] = bytes
	}

	return json.Marshal(rawMap)
}

type MonitorAttributes struct {
	Module     mariadbv1alpha1.MonitorModule `json:"module"`
	State      string                        `json:"state,omitempty"`
	Parameters MonitorParameters             `json:"parameters"`
}

type MonitorClient struct {
	GenericClient[MonitorAttributes]
}

func NewMonitorClient(client *mdbhttp.Client) *MonitorClient {
	return &MonitorClient{
		GenericClient: NewGenericClient[MonitorAttributes](
			client,
			"monitors",
			ObjectTypeMonitors,
		),
	}
}
