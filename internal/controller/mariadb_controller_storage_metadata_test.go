package controller

import (
	"fmt"
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/builder"
	pvcctrl "github.com/mariadb-operator/mariadb-operator/v26/pkg/controller/pvc"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/metadata"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcilePVCMetadata(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			mdb := &mariadbv1alpha1.MariaDB{
				ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
				Spec: mariadbv1alpha1.MariaDBSpec{
					Replicas: 3,
					Storage:  mariadbv1alpha1.Storage{Size: ptr.To(resource.MustParse("1Gi"))},
					Galera:   &mariadbv1alpha1.Galera{Enabled: true},
				},
			}
			mdb.Spec.Storage.SetDefaults()
			mdb.Spec.Galera.Config.SetDefaults()
			for _, tpl := range []*mariadbv1alpha1.VolumeClaimTemplate{
				mdb.Spec.Storage.VolumeClaimTemplate, mdb.Spec.Galera.Config.VolumeClaimTemplate,
			} {
				tpl.Metadata = &mariadbv1alpha1.Metadata{
					Labels:      map[string]string{"remove": "old", "change": "old"},
					Annotations: map[string]string{"remove": "old", "change": "old"},
				}
			}
			sts := &appsv1.StatefulSet{
				ObjectMeta: mdb.ObjectMeta,
				Spec: appsv1.StatefulSetSpec{
					VolumeClaimTemplates: builder.MariaDBVolumeClaimTemplates(mdb),
				},
			}
			if legacy {
				for i := range sts.Spec.VolumeClaimTemplates {
					delete(sts.Spec.VolumeClaimTemplates[i].Annotations, metadata.PVCLabelsAnnotation)
					delete(sts.Spec.VolumeClaimTemplates[i].Annotations, metadata.PVCAnnotationsAnnotation)
				}
			}
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, appsv1.AddToScheme(scheme))
			k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).Build()
			r := &MariaDBReconciler{Client: k8sClient, PVCReconciler: pvcctrl.NewPVCReconciler(k8sClient)}
			createClaims := func(index int) []corev1.PersistentVolumeClaim {
				t.Helper()
				var claims []corev1.PersistentVolumeClaim
				for _, template := range sts.Spec.VolumeClaimTemplates {
					claim := template.DeepCopy()
					claim.Name = fmt.Sprintf("%s-db-%d", template.Name, index)
					claim.Namespace = mdb.Namespace
					claim.Labels["external"] = "keep"
					claim.Annotations["pv.kubernetes.io/bind-completed"] = "yes"
					claim.Spec.VolumeName = "bound-volume"
					require.NoError(t, k8sClient.Create(t.Context(), claim))
					claims = append(claims, *claim)
				}
				return claims
			}
			claims := createClaims(0)
			unrelated := claims[0].DeepCopy()
			unrelated.Name = "storage-db-unrelated"
			unrelated.ResourceVersion = ""
			require.NoError(t, k8sClient.Create(t.Context(), unrelated))
			mdb.Spec.InheritMetadata = &mariadbv1alpha1.Metadata{
				Labels: map[string]string{"inherited": "yes", "change": "inherited"},
			}
			for _, tpl := range []*mariadbv1alpha1.VolumeClaimTemplate{
				mdb.Spec.Storage.VolumeClaimTemplate, mdb.Spec.Galera.Config.VolumeClaimTemplate,
			} {
				tpl.Metadata = &mariadbv1alpha1.Metadata{
					Labels:      map[string]string{"velero.io/exclude-from-backup": "true", "change": "new"},
					Annotations: map[string]string{"change": "new"},
				}
			}
			_, err := r.reconcileStorage(t.Context(), mdb)
			require.NoError(t, err)
			for _, original := range claims {
				var claim corev1.PersistentVolumeClaim
				require.NoError(t, k8sClient.Get(t.Context(), client.ObjectKeyFromObject(&original), &claim))
				require.Equal(t, "new", claim.Labels["change"])
				require.Equal(t, "new", claim.Annotations["change"])
				require.Equal(t, "true", claim.Labels["velero.io/exclude-from-backup"])
				require.Equal(t, "yes", claim.Labels["inherited"])
				require.NotContains(t, claim.Labels, "remove")
				require.NotContains(t, claim.Annotations, "remove")
				require.Equal(t, original.Spec, claim.Spec)
			}

			mdb.Spec.InheritMetadata = nil
			mdb.Spec.Storage.VolumeClaimTemplate.Metadata = nil
			mdb.Spec.Galera.Config.VolumeClaimTemplate.Metadata = nil
			// Scale-out can still create claims from the immutable, original StatefulSet template.
			claims = append(claims, createClaims(3)...)
			_, err = r.reconcileStorage(t.Context(), mdb)
			require.NoError(t, err)
			for _, original := range claims {
				var claim corev1.PersistentVolumeClaim
				key := client.ObjectKeyFromObject(&original)
				require.NoError(t, k8sClient.Get(t.Context(), key, &claim))
				require.Equal(t, map[string]string{
					"app.kubernetes.io/name": "mariadb", "app.kubernetes.io/instance": "db",
					"pvc.k8s.mariadb.com/role": original.Labels["pvc.k8s.mariadb.com/role"], "external": "keep",
				}, claim.Labels)
				require.Len(t, claim.Annotations, 3)
				require.Equal(t, "yes", claim.Annotations["pv.kubernetes.io/bind-completed"])
				require.Equal(t, original.Spec, claim.Spec)
				version := claim.ResourceVersion
				_, err = r.reconcileStorage(t.Context(), mdb)
				require.NoError(t, err)
				require.NoError(t, k8sClient.Get(t.Context(), key, &claim))
				require.Equal(t, version, claim.ResourceVersion, "unchanged metadata must not be patched")
			}
			var actualSts appsv1.StatefulSet
			require.NoError(t, k8sClient.Get(t.Context(), client.ObjectKeyFromObject(sts), &actualSts))
			require.Equal(t, sts.Spec, actualSts.Spec)
			var actualUnrelated corev1.PersistentVolumeClaim
			require.NoError(t, k8sClient.Get(t.Context(), client.ObjectKeyFromObject(unrelated), &actualUnrelated))
			require.Equal(t, unrelated, &actualUnrelated)
		})
	}
}
