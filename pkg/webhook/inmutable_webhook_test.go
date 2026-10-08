package webhook_test

import (
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/webhook"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var objectMeta = metav1.ObjectMeta{
	Name: "test",
}

var _ = Describe("InmutableWebhook", func() {
	DescribeTable("validating updates",
		func(old, new client.Object, wantErr bool) {
			inmutableWebhook := webhook.NewInmutableWebhook(
				webhook.WithTagName("webhook"),
			)
			err := inmutableWebhook.ValidateUpdate(new, old)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("mutable",
			&mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					BackoffLimit: 10,
				},
			},
			&mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					BackoffLimit: 20,
				},
			},
			false,
		),
		Entry("inmutable",
			&mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
			&mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
			true,
		),
		Entry("mutable nested struct",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Monitor: mariadbv1alpha1.MaxScaleMonitor{
						Interval: metav1.Duration{Duration: 2 * time.Second},
					},
				},
			},
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Monitor: mariadbv1alpha1.MaxScaleMonitor{
						Interval: metav1.Duration{Duration: 5 * time.Second},
					},
				},
			},
			false,
		),
		Entry("inmutable nested struct",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Monitor: mariadbv1alpha1.MaxScaleMonitor{
						Module: mariadbv1alpha1.MonitorModuleMariadb,
					},
				},
			},
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Monitor: mariadbv1alpha1.MaxScaleMonitor{
						Module: mariadbv1alpha1.MonitorModuleGalera,
					},
				},
			},
			true,
		),
		Entry("mutable nested pointer to struct",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: false,
					},
				},
			},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Enabled: true,
					},
				},
			},
			false,
		),
		Entry("inmutable nested pointer to struct",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Username: "foo",
					},
				},
			},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					Metrics: &mariadbv1alpha1.MariadbMetrics{
						Username: "bar",
					},
				},
			},
			true,
		),
		Entry("mutable nested slice",
			&mariadbv1alpha1.MaxScale{
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
			&mariadbv1alpha1.MaxScale{
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
			false,
		),
		Entry("inmutable nested slice",
			&mariadbv1alpha1.MaxScale{
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
			&mariadbv1alpha1.MaxScale{
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
			true,
		),
		Entry("inmutable nested slice adding elements",
			&mariadbv1alpha1.MaxScale{
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
			&mariadbv1alpha1.MaxScale{
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
			true,
		),
		Entry("mutable nested struct in slice",
			&mariadbv1alpha1.MaxScale{
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
			&mariadbv1alpha1.MaxScale{
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
			false,
		),
		Entry("inmutable nested struct in slice",
			&mariadbv1alpha1.MaxScale{
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
			&mariadbv1alpha1.MaxScale{
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
			true,
		),
	)
})

var _ = Describe("InmutableInitWebhook", func() {
	DescribeTable("validating updates",
		func(old, new client.Object, wantErr bool) {
			inmutableWebhook := webhook.NewInmutableWebhook(
				webhook.WithTagName("webhook"),
			)
			err := inmutableWebhook.ValidateUpdate(new, old)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("nil field",
			&mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: nil,
					},
				},
			},
			&mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: &mariadbv1alpha1.LocalObjectReference{
							Name: "bar",
						},
					},
				},
			},
			false,
		),
		Entry("non nil field",
			&mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: &mariadbv1alpha1.LocalObjectReference{
							Name: "bar",
						},
					},
				},
			},
			&mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: &mariadbv1alpha1.LocalObjectReference{
							Name: "foo",
						},
					},
				},
			},
			true,
		),
		Entry("zero field",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{},
				},
			},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
							LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
								Name: "mariadb-root",
							},
							Key: "password",
						},
						Generate: true,
					},
				},
			},
			false,
		),
		Entry("non zero field",
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
							LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
								Name: "mariadb-root",
							},
							Key: "password",
						},
					},
				},
			},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MariaDBSpec{
					RootPasswordSecretKeyRef: mariadbv1alpha1.GeneratedSecretKeyRef{
						SecretKeySelector: mariadbv1alpha1.SecretKeySelector{
							LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
								Name: "mariadb-root",
							},
							Key: "another-password",
						},
					},
				},
			},
			true,
		),
		Entry("struct",
			&mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: &mariadbv1alpha1.LocalObjectReference{
							Name: "foo",
						},
					},
				},
			},
			&mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestoreSource: mariadbv1alpha1.RestoreSource{
						BackupRef: &mariadbv1alpha1.LocalObjectReference{
							Name: "foo",
						},
						Volume: &mariadbv1alpha1.StorageVolumeSource{
							NFS: &mariadbv1alpha1.NFSVolumeSource{
								Server: "nas.local",
								Path:   "/volume/foo",
							},
						},
					},
				},
			},
			false,
		),
	)
})

var _ = Describe("InmutableWebhookError", func() {
	DescribeTable("validating update error fields",
		func(old, new client.Object, wantFields []string) {
			inmutableWebhook := webhook.NewInmutableWebhook(
				webhook.WithTagName("webhook"),
			)
			err := inmutableWebhook.ValidateUpdate(new, old)
			Expect(err).To(HaveOccurred())

			apiErr, ok := err.(*apierrors.StatusError)
			Expect(ok).To(BeTrue())

			fields := make([]string, len(apiErr.ErrStatus.Details.Causes))
			for i, c := range apiErr.ErrStatus.Details.Causes {
				fields[i] = c.Field
			}

			Expect(fields).To(Equal(wantFields))
		},
		Entry("root field",
			&mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
			&mariadbv1alpha1.Restore{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.RestoreSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
			[]string{
				"spec.restartPolicy",
			},
		),
		Entry("nested struct",
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Monitor: mariadbv1alpha1.MaxScaleMonitor{
						Module: mariadbv1alpha1.MonitorModuleMariadb,
					},
				},
			},
			&mariadbv1alpha1.MaxScale{
				ObjectMeta: objectMeta,
				Spec: mariadbv1alpha1.MaxScaleSpec{
					Monitor: mariadbv1alpha1.MaxScaleMonitor{
						Module: mariadbv1alpha1.MonitorModuleGalera,
					},
				},
			},
			[]string{
				"spec.monitor.module",
			},
		),
		Entry("nested slice",
			&mariadbv1alpha1.MaxScale{
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
			&mariadbv1alpha1.MaxScale{
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
			[]string{
				"spec.services.router",
			},
		),
		Entry("nested struct in slice",
			&mariadbv1alpha1.MaxScale{
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
			&mariadbv1alpha1.MaxScale{
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
			[]string{
				"spec.services.listener.port",
			},
		),
	)
})
