package config

import (
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
)

const (
	// Fraction of container memory limit used to calculate maximum size of query classifier cache.
	// Value taken from https://mariadb.com/kb/en/mariadb-maxscale-6-mariadb-maxscale-configuration-guide/#query_classifier_cache_size
	queryClassifierCacheLimitFraction = 0.15
)

func threads(mxs *mariadbv1alpha1.MaxScale) string {
	threads := "auto"
	if mxs.Spec.Resources == nil || mxs.Spec.Resources.Limits == nil {
		return threads
	}

	cpuLimit := mxs.Spec.Resources.Limits.Cpu().Value()
	if cpuLimit != 0 {
		threads = fmt.Sprintf("%d", cpuLimit)
	}
	return threads
}

func queryClassifierCacheSize(mxs *mariadbv1alpha1.MaxScale) string {
	queryClassifierCacheSize := ""
	if mxs.Spec.Resources == nil || mxs.Spec.Resources.Limits == nil {
		return queryClassifierCacheSize
	}

	memLimit := mxs.Spec.Resources.Limits.Memory().Value()
	if memLimit != 0 {
		queryClassCacheScaled := int64(float64(memLimit) * queryClassifierCacheLimitFraction)
		queryClassifierCacheSize = fmt.Sprintf("%d", queryClassCacheScaled)
	}
	return queryClassifierCacheSize
}
