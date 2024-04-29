package webhook_test

import (
	"reflect"
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/webhook"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestInmutableWebhook(t *testing.T) {
	inmutableWebhook := webhook.NewInmutableWebhook(
		webhook.WithTagName("webhook"),
	)
	objectMeta := metav1.ObjectMeta{
		Name: "test",
	}

	tests := []struct {
		name    string
		old     client.Object
		new     client.Object
		wantErr bool
	}{
		{
			name: "mutable",
			old: &mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					BackoffLimit: 10,
				},
			},
			new: &mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					BackoffLimit: 20,
				},
			},
			wantErr: false,
		},
		{
			name: "inmutable",
			old: &mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
			new: &mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
			wantErr: true,
		},
		{
			name: "mutable nested struct",
			old: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Monitor: mariadbv1alpha1.MaxScaleMonitor{
						Interval: metav1.Duration{Duration: 2 * time.Second},
					},
				},
			},
			new: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Monitor: mariadbv1alpha1.MaxScaleMonitor{
						Interval: metav1.Duration{Duration: 5 * time.Second},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "inmutable nested struct",
			old: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Monitor: mariadbv1alpha1.MaxScaleMonitor{
						Module: mariadbv1alpha1.MonitorModuleMariadb,
					},
				},
			},
			new: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Monitor: mariadbv1alpha1.MaxScaleMonitor{
						Module: mariadbv1alpha1.MonitorModuleGalera,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "mutable nested pointer to struct",
			old: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: false,
					},
				},
			},
			new: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "inmutable nested pointer to struct",
			old: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Username: "foo",
					},
				},
			},
			new: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Username: "bar",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "mutable nested slice",
			old: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name: "foo",
							Params: map[string]string{
								"test": "foo",
							},
						},
					},
				},
			},
			new: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name: "foo",
							Params: map[string]string{
								"test": "bar",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "inmutable nested slice",
			old: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name:   "foo",
							Router: mariadbv1alpha1.ServiceRouterReadConnRoute,
						},
					},
				},
			},
			new: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name:   "foo",
							Router: mariadbv1alpha1.ServiceRouterReadWriteSplit,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "inmutable nested slice adding elements",
			old: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name:   "foo",
							Router: mariadbv1alpha1.ServiceRouterReadConnRoute,
						},
					},
				},
			},
			new: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name:   "foo",
							Router: mariadbv1alpha1.ServiceRouterReadWriteSplit,
						},
						{
							Name:   "bar",
							Router: mariadbv1alpha1.ServiceRouterReadWriteSplit,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "mutable nested struct in slice",
			old: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name: "foo",
							Listener: mariadbv1alpha1.MaxScaleListener{
								Protocol: "foo",
							},
						},
					},
				},
			},
			new: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name: "foo",
							Listener: mariadbv1alpha1.MaxScaleListener{
								Protocol: "bar",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "inmutable nested struct in slice",
			old: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name: "foo",
							Listener: mariadbv1alpha1.MaxScaleListener{
								Port: 1234,
							},
						},
					},
				},
			},
			new: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name: "foo",
							Listener: mariadbv1alpha1.MaxScaleListener{
								Port: 5678,
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := inmutableWebhook.ValidateUpdate(tt.new, tt.old)
			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
		})
	}
}

func TestInmutableInitWebhook(t *testing.T) {
	inmutableWebhook := webhook.NewInmutableWebhook(
		webhook.WithTagName("webhook"),
	)
	objectMeta := metav1.ObjectMeta{
		Name: "test",
	}

	tests := []struct {
		name    string
		old     client.Object
		new     client.Object
		wantErr bool
	}{
		{
			name: "nil field",
			old: &mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: nil,
					},
				},
			},
			new: &mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "bar",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "non nil field",
			old: &mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "bar",
						},
					},
				},
			},
			new: &mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "foo",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "zero field",
			old: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{},
				},
			},
			new: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-root",
							},
							Key: "password",
						},
						Generate: true,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "non zero field",
			old: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-root",
							},
							Key: "password",
						},
					},
				},
			},
			new: &mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "mariadb-root",
							},
							Key: "another-password",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "struct",
			old: &mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "foo",
						},
					},
				},
			},
			new: &mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "foo",
						},
						Volume: &corev1.VolumeSource{
							NFS: &corev1.NFSVolumeSource{
								Server: "nas.local",
								Path:   "/volume/foo",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := inmutableWebhook.ValidateUpdate(tt.new, tt.old)
			if tt.wantErr && err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expect error to not have occurred, got: %v", err)
			}
		})
	}
}

func TestInmutableWebhookError(t *testing.T) {
	inmutableWebhook := webhook.NewInmutableWebhook(
		webhook.WithTagName("webhook"),
	)
	objectMeta := metav1.ObjectMeta{
		Name: "test",
	}

	tests := []struct {
		name       string
		old        client.Object
		new        client.Object
		wantFields []string
	}{
		{
			name: "root field",
			old: &mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
			new: &mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
			wantFields: []string{
				"spec.restartPolicy",
			},
		},
		{
			name: "nested struct",
			old: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Monitor: mariadbv1alpha1.MaxScaleMonitor{
						Module: mariadbv1alpha1.MonitorModuleMariadb,
					},
				},
			},
			new: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Monitor: mariadbv1alpha1.MaxScaleMonitor{
						Module: mariadbv1alpha1.MonitorModuleGalera,
					},
				},
			},
			wantFields: []string{
				"spec.monitor.module",
			},
		},
		{
			name: "nested slice",
			old: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name:   "foo",
							Router: mariadbv1alpha1.ServiceRouterReadConnRoute,
						},
					},
				},
			},
			new: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name:   "foo",
							Router: mariadbv1alpha1.ServiceRouterReadWriteSplit,
						},
					},
				},
			},
			wantFields: []string{
				"spec.services.router",
			},
		},
		{
			name: "nested struct in slice",
			old: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name: "foo",
							Listener: mariadbv1alpha1.MaxScaleListener{
								Port: 1234,
							},
						},
					},
				},
			},
			new: &mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Services: []mariadbv1alpha1.MaxScaleService{
						{
							Name: "foo",
							Listener: mariadbv1alpha1.MaxScaleListener{
								Port: 5678,
							},
						},
					},
				},
			},
			wantFields: []string{
				"spec.services.listener.port",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := inmutableWebhook.ValidateUpdate(tt.new, tt.old)
			if err == nil {
				t.Error("expect error to have occurred, got nil")
			}
			apiErr, ok := err.(*apierrors.StatusError)
			if !ok {
				t.Errorf("unable to cast error to API error: %v", err)
			}
			fields := make([]string, len(apiErr.ErrStatus.Details.Causes))
			for i, c := range apiErr.ErrStatus.Details.Causes {
				fields[i] = c.Field
			}

			if !reflect.DeepEqual(tt.wantFields, fields) {
				t.Errorf("expect error to be: '%s', got: %v", tt.wantFields, fields)
			}
		})
	}
}
