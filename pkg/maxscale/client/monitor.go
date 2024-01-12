package client

import (
	"context"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	mdbhttp "github.com/mariadb-operator/mariadb-operator/pkg/http"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MonitorParameters struct {
	ExtraParams     map[string]string `json:",inline"`
	User            string            `json:"user"`
	Password        string            `json:"password"`
	MonitorInterval metav1.Duration   `json:"monitor_interval,omitempty"`
}

type MonitorAttributes struct {
	Module     mariadbv1alpha1.MonitorModule `json:"module"`
	Parameters MonitorParameters             `json:"parameters"`
}

type MonitorClient struct {
	client *mdbhttp.Client
}

func (m *MonitorClient) Create(ctx context.Context, module mariadbv1alpha1.MonitorModule, params MonitorParameters) error {
	object := &Object[MonitorAttributes]{
		Data: Data[MonitorAttributes]{
			ID:   string(module),
			Type: ObjectTypeMonitors,
			Attributes: MonitorAttributes{
				Module:     module,
				Parameters: params,
			},
		},
	}
	res, err := m.client.Post(ctx, "monitors", object, nil)
	if err != nil {
		return err
	}
	return handleResponse(res, nil)
}
