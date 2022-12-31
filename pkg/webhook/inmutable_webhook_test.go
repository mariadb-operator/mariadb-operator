package webhook_test

import (
	"testing"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/pkg/webhook"
	corev1 "k8s.io/api/core/v1"
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
			name: "update mutable field",
			old: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					BackoffLimit: 10,
				},
			},
			new: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					BackoffLimit: 20,
				},
			},
			wantErr: false,
		},
		{
			name: "update inmutable field",
			old: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
			new: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
			wantErr: true,
		},
		{
			name: "update inmutableinit primitive field",
			old: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestoreSource: databasev1alpha1.RestoreSource{
						Physical: func() *bool {
							p := true
							return &p
						}(),
					},
				},
			},
			new: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestoreSource: databasev1alpha1.RestoreSource{
						Physical: func() *bool {
							p := false
							return &p
						}(),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "init inmutableinit primitive field",
			old: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestoreSource: databasev1alpha1.RestoreSource{
						Physical: nil,
					},
				},
			},
			new: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestoreSource: databasev1alpha1.RestoreSource{
						Physical: func() *bool {
							p := false
							return &p
						}(),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "update inmutableinit object field",
			old: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestoreSource: databasev1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "foo",
						},
					},
				},
			},
			new: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestoreSource: databasev1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "bar",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "init inmutableinit object field",
			old: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestoreSource: databasev1alpha1.RestoreSource{
						BackupRef: nil,
					},
				},
			},
			new: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestoreSource: databasev1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "bar",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "controller init",
			old: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestoreSource: databasev1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "foo",
						},
					},
				},
			},
			new: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestoreSource: databasev1alpha1.RestoreSource{
						BackupRef: &corev1.LocalObjectReference{
							Name: "foo",
						},
						Volume: &corev1.VolumeSource{
							NFS: &corev1.NFSVolumeSource{
								Server: "nas.local",
								Path:   "/volume/foo",
							},
						},
						Physical: func() *bool {
							p := true
							return &p
						}(),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "complex update",
			old: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestoreSource: databasev1alpha1.RestoreSource{
						Volume: &corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "foo",
							},
						},
						Physical: func() *bool {
							p := false
							return &p
						}(),
					},
					MariaDBRef: databasev1alpha1.MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "foo",
						},
					},
					BackoffLimit: 10,
				},
			},
			new: &databasev1alpha1.RestoreMariaDB{
				ObjectMeta: objectMeta,
				Spec: databasev1alpha1.RestoreMariaDBSpec{
					RestoreSource: databasev1alpha1.RestoreSource{
						Volume: &corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "foo",
							},
						},
						Physical: func() *bool {
							p := false
							return &p
						}(),
					},
					MariaDBRef: databasev1alpha1.MariaDBRef{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "foo",
						},
					},
					BackoffLimit: 20,
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
