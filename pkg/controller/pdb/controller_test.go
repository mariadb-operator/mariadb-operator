package pdb

import (
	"context"
	"testing"

	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	key      = types.NamespacedName{Name: "mariadb", Namespace: "default"}
	selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "mariadb"}}
	owner    = &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: "default", UID: "owner-uid"}}
)

func newPDB(minAvailable, maxUnavailable *intstr.IntOrString, controlled bool) *policyv1.PodDisruptionBudget {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable:   minAvailable,
			MaxUnavailable: maxUnavailable,
			Selector:       selector,
		},
	}
	if controlled {
		pdb.OwnerReferences = []metav1.OwnerReference{
			{Name: owner.Name, UID: owner.UID, Controller: ptr.To(true)},
		}
	}
	return pdb
}

func newReconciler(objs ...client.Object) (*PDBReconciler, client.Client) {
	scheme := runtime.NewScheme()
	if err := policyv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return NewPDBReconciler(c), c
}

func TestReconcile(t *testing.T) {
	one := intstr.FromInt32(1)
	two := intstr.FromInt32(2)

	tests := []struct {
		name         string
		existing     *policyv1.PodDisruptionBudget
		desired      *policyv1.PodDisruptionBudget
		wantMin      *intstr.IntOrString
		wantMaxUnav  *intstr.IntOrString
		wantSelector *metav1.LabelSelector
	}{
		{
			name:         "creates when missing",
			desired:      newPDB(&one, nil, false),
			wantMin:      &one,
			wantSelector: selector,
		},
		{
			name:         "patches minAvailable",
			existing:     newPDB(&one, nil, false),
			desired:      newPDB(&two, nil, false),
			wantMin:      &two,
			wantSelector: selector,
		},
		{
			name:         "switches minAvailable to maxUnavailable",
			existing:     newPDB(&one, nil, false),
			desired:      newPDB(nil, &one, false),
			wantMaxUnav:  &one,
			wantSelector: selector,
		},
		{
			name:         "switches maxUnavailable to minAvailable",
			existing:     newPDB(nil, &one, false),
			desired:      newPDB(&two, nil, false),
			wantMin:      &two,
			wantSelector: selector,
		},
		{
			name:     "keeps equal spec and does not touch the selector",
			existing: newPDB(&one, nil, false),
			desired: &policyv1.PodDisruptionBudget{ObjectMeta: newPDB(nil, nil, false).ObjectMeta, Spec: policyv1.PodDisruptionBudgetSpec{
				MinAvailable: &one,
			}},
			wantMin:      &one,
			wantSelector: selector,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []client.Object
			if tt.existing != nil {
				objs = append(objs, tt.existing)
			}
			r, c := newReconciler(objs...)

			if err := r.Reconcile(context.Background(), tt.desired); err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}

			var got policyv1.PodDisruptionBudget
			if err := c.Get(context.Background(), key, &got); err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if !equal(got.Spec.MinAvailable, tt.wantMin) || !equal(got.Spec.MaxUnavailable, tt.wantMaxUnav) {
				t.Errorf("spec = min %v max %v, want min %v max %v",
					got.Spec.MinAvailable, got.Spec.MaxUnavailable, tt.wantMin, tt.wantMaxUnav)
			}
			if got.Spec.Selector.MatchLabels["app"] != tt.wantSelector.MatchLabels["app"] {
				t.Errorf("selector = %v, want %v", got.Spec.Selector, tt.wantSelector)
			}
		})
	}
}

func TestDeleteControlled(t *testing.T) {
	one := intstr.FromInt32(1)

	tests := []struct {
		name        string
		existing    *policyv1.PodDisruptionBudget
		wantDeleted bool
	}{
		{name: "missing PDB is a no-op", existing: nil, wantDeleted: true},
		{name: "deletes controlled PDB", existing: newPDB(&one, nil, true), wantDeleted: true},
		{name: "keeps PDB not controlled by the owner", existing: newPDB(&one, nil, false), wantDeleted: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []client.Object
			if tt.existing != nil {
				objs = append(objs, tt.existing)
			}
			r, c := newReconciler(objs...)

			if err := r.DeleteControlled(context.Background(), key, owner); err != nil {
				t.Fatalf("DeleteControlled() error = %v", err)
			}

			var got policyv1.PodDisruptionBudget
			err := c.Get(context.Background(), key, &got)
			if tt.wantDeleted && !apierrors.IsNotFound(err) {
				t.Errorf("expected PDB to be deleted, Get() error = %v", err)
			}
			if !tt.wantDeleted && err != nil {
				t.Errorf("expected PDB to be kept, Get() error = %v", err)
			}
		})
	}
}

func equal(a, b *intstr.IntOrString) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func TestReconcileMetadata(t *testing.T) {
	one := intstr.FromInt32(1)
	existing := newPDB(&one, nil, false)
	existing.Labels = map[string]string{"foreign": "keep", "inherited": "old"}
	desired := newPDB(&one, nil, false)
	desired.Labels = map[string]string{"inherited": "new"}
	desired.Annotations = map[string]string{"note": "added"}

	r, c := newReconciler(existing)
	if err := r.Reconcile(context.Background(), desired); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var got policyv1.PodDisruptionBudget
	if err := c.Get(context.Background(), key, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Labels["inherited"] != "new" || got.Labels["foreign"] != "keep" || got.Annotations["note"] != "added" {
		t.Errorf("metadata = labels %v annotations %v", got.Labels, got.Annotations)
	}
}
