package predicate

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func PredicateWithAnnotations(annotations []string) predicate.Predicate {
	return PredicateChangedWithAnnotations(annotations, func(old, new client.Object) bool {
		return true
	})
}

func PredicateWithLabel(label string) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return hasLabel(e.Object, label)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return hasLabel(e.Object, label)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return hasLabel(e.ObjectNew, label)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return hasLabel(e.Object, label)
		},
	}
}

func PredicateChangedWithAnnotations(annotations []string, hasChanged func(old, new client.Object) bool) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return hasAnnotations(e.Object, annotations)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !hasAnnotations(e.ObjectOld, annotations) || !hasAnnotations(e.ObjectNew, annotations) {
				return false
			}
			return hasChanged(e.ObjectOld, e.ObjectNew)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return hasAnnotations(e.Object, annotations)
		},
	}
}

func hasAnnotations(o client.Object, annotations []string) bool {
	objAnnotations := o.GetAnnotations()
	for _, a := range annotations {
		if _, ok := objAnnotations[a]; !ok {
			return false
		}
	}
	return true
}

func hasLabel(o client.Object, label string) bool {
	_, hasLabel := o.GetLabels()[label]
	return hasLabel
}
