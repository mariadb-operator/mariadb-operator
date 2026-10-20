package metadata

import (
	"maps"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PVCLabelsAnnotation      = "k8s.mariadb.com/pvc-labels"
	PVCAnnotationsAnnotation = "k8s.mariadb.com/pvc-annotations"
)

// SetPVCMetadata tracks managed keys so removing template metadata preserves storage-driver metadata.
func SetPVCMetadata(existing *metav1.ObjectMeta, desired metav1.ObjectMeta) {
	if existing.Labels == nil {
		existing.Labels = make(map[string]string)
	}
	if existing.Annotations == nil {
		existing.Annotations = make(map[string]string)
	}
	for _, key := range strings.Split(existing.Annotations[PVCLabelsAnnotation], ",") {
		delete(existing.Labels, key)
	}
	for _, key := range strings.Split(existing.Annotations[PVCAnnotationsAnnotation], ",") {
		delete(existing.Annotations, key)
	}
	maps.Copy(existing.Labels, desired.Labels)
	maps.Copy(existing.Annotations, desired.Annotations)
	annotationKeys := maps.Clone(desired.Annotations)
	delete(annotationKeys, PVCLabelsAnnotation)
	delete(annotationKeys, PVCAnnotationsAnnotation)
	existing.Annotations[PVCLabelsAnnotation] = strings.Join(slices.Sorted(maps.Keys(desired.Labels)), ",")
	existing.Annotations[PVCAnnotationsAnnotation] = strings.Join(slices.Sorted(maps.Keys(annotationKeys)), ",")
}
