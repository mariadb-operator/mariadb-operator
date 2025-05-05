// nolint:lll
package v1alpha1

import (
	kadapter "github.com/mariadb-operator/mariadb-operator/pkg/kubernetes/adapter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselectorrequirement-v1-meta
type LabelSelectorRequirement struct {
	Key      string                       `json:"key"`
	Operator metav1.LabelSelectorOperator `json:"operator"`
	// +optional
	// +listType=atomic
	Values []string `json:"values,omitempty"`
}

func (s LabelSelectorRequirement) ToKubernetesType() metav1.LabelSelectorRequirement {
	return metav1.LabelSelectorRequirement{
		Key:      s.Key,
		Operator: s.Operator,
		Values:   s.Values,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta
type LabelSelector struct {
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
	// +optional
	// +listType=atomic
	MatchExpressions []LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

func (s LabelSelector) ToKubernetesType() metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchLabels:      s.MatchLabels,
		MatchExpressions: kadapter.ToKubernetesSlice(s.MatchExpressions),
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podaffinityterm-v1-core.
type PodAffinityTerm struct {
	// +optional
	LabelSelector *LabelSelector `json:"labelSelector,omitempty"`
	TopologyKey   string         `json:"topologyKey"`
}

func (p PodAffinityTerm) ToKubernetesType() corev1.PodAffinityTerm {
	affinityTerm := corev1.PodAffinityTerm{
		TopologyKey: p.TopologyKey,
	}
	if p.LabelSelector != nil {
		affinityTerm.LabelSelector = ptr.To(p.LabelSelector.ToKubernetesType())
	}
	return affinityTerm
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#weightedpodaffinityterm-v1-core.
type WeightedPodAffinityTerm struct {
	Weight          int32           `json:"weight"`
	PodAffinityTerm PodAffinityTerm `json:"podAffinityTerm"`
}

func (p WeightedPodAffinityTerm) ToKubernetesType() corev1.WeightedPodAffinityTerm {
	return corev1.WeightedPodAffinityTerm{
		Weight:          p.Weight,
		PodAffinityTerm: p.PodAffinityTerm.ToKubernetesType(),
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podantiaffinity-v1-core.
type PodAntiAffinity struct {
	// +optional
	// +listType=atomic
	RequiredDuringSchedulingIgnoredDuringExecution []PodAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	// +optional
	// +listType=atomic
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

func (p PodAntiAffinity) ToKubernetesType() corev1.PodAntiAffinity {
	return corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution:  kadapter.ToKubernetesSlice(p.RequiredDuringSchedulingIgnoredDuringExecution),
		PreferredDuringSchedulingIgnoredDuringExecution: kadapter.ToKubernetesSlice(p.PreferredDuringSchedulingIgnoredDuringExecution),
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#nodeselectorrequirement-v1-core
type NodeSelectorRequirement struct {
	Key      string                      `json:"key"`
	Operator corev1.NodeSelectorOperator `json:"operator"`
	// +optional
	// +listType=atomic
	Values []string `json:"values,omitempty"`
}

func (s NodeSelectorRequirement) ToKubernetesType() corev1.NodeSelectorRequirement {
	return corev1.NodeSelectorRequirement{
		Key:      s.Key,
		Operator: s.Operator,
		Values:   s.Values,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#nodeselectorterm-v1-core
type NodeSelectorTerm struct {
	// +optional
	// +listType=atomic
	MatchExpressions []NodeSelectorRequirement `json:"matchExpressions,omitempty"`
	// +optional
	// +listType=atomic
	MatchFields []NodeSelectorRequirement `json:"matchFields,omitempty"`
}

func (s NodeSelectorTerm) ToKubernetesType() corev1.NodeSelectorTerm {
	return corev1.NodeSelectorTerm{
		MatchExpressions: kadapter.ToKubernetesSlice(s.MatchExpressions),
		MatchFields:      kadapter.ToKubernetesSlice(s.MatchFields),
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#nodeselector-v1-core
type NodeSelector struct {
	// +listType=atomic
	NodeSelectorTerms []NodeSelectorTerm `json:"nodeSelectorTerms"`
}

func (p NodeSelector) ToKubernetesType() corev1.NodeSelector {
	return corev1.NodeSelector{
		NodeSelectorTerms: kadapter.ToKubernetesSlice(p.NodeSelectorTerms),
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#preferredschedulingterm-v1-core
type PreferredSchedulingTerm struct {
	Weight     int32            `json:"weight"`
	Preference NodeSelectorTerm `json:"preference"`
}

func (p PreferredSchedulingTerm) ToKubernetesType() corev1.PreferredSchedulingTerm {
	return corev1.PreferredSchedulingTerm{
		Weight:     p.Weight,
		Preference: p.Preference.ToKubernetesType(),
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#nodeaffinity-v1-core
type NodeAffinity struct {
	// +optional
	RequiredDuringSchedulingIgnoredDuringExecution *NodeSelector `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	// +optional
	// +listType=atomic
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

func (p NodeAffinity) ToKubernetesType() corev1.NodeAffinity {
	nodeAffinity := corev1.NodeAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: kadapter.ToKubernetesSlice(p.PreferredDuringSchedulingIgnoredDuringExecution),
	}
	if p.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = ptr.To(p.RequiredDuringSchedulingIgnoredDuringExecution.ToKubernetesType())
	}
	return nodeAffinity
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core.
type Affinity struct {
	// +optional
	PodAntiAffinity *PodAntiAffinity `json:"podAntiAffinity,omitempty"`
	// +optional
	NodeAffinity *NodeAffinity `json:"nodeAffinity,omitempty"`
}

func (a Affinity) ToKubernetesType() corev1.Affinity {
	var affinity corev1.Affinity
	if a.PodAntiAffinity != nil {
		affinity.PodAntiAffinity = ptr.To(a.PodAntiAffinity.ToKubernetesType())
	}
	if a.NodeAffinity != nil {
		affinity.NodeAffinity = ptr.To(a.NodeAffinity.ToKubernetesType())
	}
	return affinity
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#topologyspreadconstraint-v1-core.
type TopologySpreadConstraint struct {
	MaxSkew           int32                                `json:"maxSkew"`
	TopologyKey       string                               `json:"topologyKey"`
	WhenUnsatisfiable corev1.UnsatisfiableConstraintAction `json:"whenUnsatisfiable"`
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	// +optional
	MinDomains *int32 `json:"minDomains,omitempty"`
	// +optional
	NodeAffinityPolicy *corev1.NodeInclusionPolicy `json:"nodeAffinityPolicy,omitempty"`
	// +optional
	NodeTaintsPolicy *corev1.NodeInclusionPolicy `json:"nodeTaintsPolicy,omitempty"`
	// +optional
	MatchLabelKeys []string `json:"matchLabelKeys,omitempty"`
}

func (t TopologySpreadConstraint) ToKubernetesType() corev1.TopologySpreadConstraint {
	return corev1.TopologySpreadConstraint{
		MaxSkew:            t.MaxSkew,
		TopologyKey:        t.TopologyKey,
		WhenUnsatisfiable:  t.WhenUnsatisfiable,
		LabelSelector:      t.LabelSelector,
		MinDomains:         t.MinDomains,
		NodeAffinityPolicy: t.NodeAffinityPolicy,
		NodeTaintsPolicy:   t.NodeTaintsPolicy,
		MatchLabelKeys:     t.MatchLabelKeys,
	}
}
