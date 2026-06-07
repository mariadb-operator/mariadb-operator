package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("SecretBuilder", func() {
	DescribeTable("Building a Secret with metadata",
		func(opts SecretOpts, wantMeta *mariadbv1alpha1.Metadata) {
			builder := newDefaultTestBuilder()
			configMap, err := builder.BuildSecret(opts, &mariadbv1alpha1.MariaDB{})
			Expect(err).NotTo(HaveOccurred())
			assertObjectMeta(&configMap.ObjectMeta, wantMeta.Labels, wantMeta.Annotations)
		},
		Entry("no meta",
			SecretOpts{
				Metadata: []*mariadbv1alpha1.Metadata{},
				Key: types.NamespacedName{
					Name: "configmap",
				},
				Data: map[string][]byte{
					"password": []byte("test"),
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		),
		Entry("single meta",
			SecretOpts{
				Metadata: []*mariadbv1alpha1.Metadata{
					{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
				Key: types.NamespacedName{
					Name: "configmap",
				},
				Data: map[string][]byte{
					"password": []byte("test"),
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io": "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
		Entry("multiple meta",
			SecretOpts{
				Metadata: []*mariadbv1alpha1.Metadata{
					{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
					{
						Labels: map[string]string{
							"sidecar.istio.io/inject": "false",
						},
					},
				},
				Key: types.NamespacedName{
					Name: "configmap",
				},
				Data: map[string][]byte{
					"password": []byte("test"),
				},
			},
			&mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					"database.myorg.io":       "mariadb",
					"sidecar.istio.io/inject": "false",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		),
	)
})

var _ = Describe("BuildSecret", func() {
	DescribeTable("Building a Secret",
		func(opts SecretOpts, owner metav1.Object, wantErr bool) {
			builder := newDefaultTestBuilder()
			secret, err := builder.BuildSecret(opts, owner)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(secret.Data).To(Equal(opts.Data))
			Expect(secret.Name).To(Equal(opts.Key.Name))
			Expect(secret.Namespace).To(Equal(opts.Key.Namespace))
			if owner != nil {
				Expect(controllerutil.HasControllerReference(secret)).To(BeTrue())
			}
		},
		Entry("no owner",
			SecretOpts{
				Metadata: []*mariadbv1alpha1.Metadata{},
				Key: types.NamespacedName{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			},
			nil,
			false,
		),
		Entry("with owner",
			SecretOpts{
				Metadata: []*mariadbv1alpha1.Metadata{},
				Key: types.NamespacedName{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			},
			&mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mariadb",
					Namespace: "default",
				},
			},
			false,
		),
	)
})
