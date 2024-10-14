// nolint:lll
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core.
type LocalObjectReference struct {
	// +optional
	// +default=""
	// +kubebuilder:default=""
	Name string `json:"name,omitempty"`
}

func (r LocalObjectReference) ToKubernetesType() corev1.LocalObjectReference {
	return corev1.LocalObjectReference{
		Name: r.Name,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectreference-v1-core.
type ObjectReference struct {
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

func (r ObjectReference) ToKubernetesType() corev1.ObjectReference {
	return corev1.ObjectReference{
		Name:      r.Name,
		Namespace: r.Namespace,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core.
// +structType=atomic
type SecretKeySelector struct {
	LocalObjectReference `json:",inline"`
	Key                  string `json:"key"`
}

func (s SecretKeySelector) ToKubernetesType() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: s.LocalObjectReference.ToKubernetesType(),
		Key:                  s.Key,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#configmapkeyselector-v1-core.
// +structType=atomic
type ConfigMapKeySelector struct {
	LocalObjectReference `json:",inline"`
	Key                  string `json:"key"`
}

func (s ConfigMapKeySelector) ToKubernetesType() corev1.ConfigMapKeySelector {
	return corev1.ConfigMapKeySelector{
		LocalObjectReference: s.LocalObjectReference.ToKubernetesType(),
		Key:                  s.Key,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectfieldselector-v1-core.
// +structType=atomic
type ObjectFieldSelector struct {
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
	FieldPath  string `json:"fieldPath"`
}

func (s ObjectFieldSelector) ToKubernetesType() corev1.ObjectFieldSelector {
	return corev1.ObjectFieldSelector{
		APIVersion: s.APIVersion,
		FieldPath:  s.FieldPath,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvarsource-v1-core.
type EnvVarSource struct {
	// +optional
	FieldRef *ObjectFieldSelector `json:"fieldRef,omitempty"`
	// +optional
	ConfigMapKeyRef *ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`
	// +optional
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`
}

func (e EnvVarSource) ToKubernetesType() corev1.EnvVarSource {
	var env corev1.EnvVarSource
	if e.FieldRef != nil {
		env.FieldRef = ptr.To(e.FieldRef.ToKubernetesType())
	}
	if e.ConfigMapKeyRef != nil {
		env.ConfigMapKeyRef = ptr.To(e.ConfigMapKeyRef.ToKubernetesType())
	}
	if e.SecretKeyRef != nil {
		env.SecretKeyRef = ptr.To(e.SecretKeyRef.ToKubernetesType())
	}
	return env
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvarsource-v1-core.
type EnvVar struct {
	// Name of the environment variable. Must be a C_IDENTIFIER.
	Name string `json:"name"`
	// +optional
	Value string `json:"value,omitempty"`
	// +optional
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

func (e EnvVar) ToKubernetesType() corev1.EnvVar {
	env := corev1.EnvVar{
		Name:  e.Name,
		Value: e.Value,
	}
	if e.ValueFrom != nil {
		env.ValueFrom = ptr.To(e.ValueFrom.ToKubernetesType())
	}
	return env
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envfromsource-v1-core.
type EnvFromSource struct {
	// +optional
	Prefix string `json:"prefix,omitempty"`
	// +optional
	ConfigMapRef *LocalObjectReference `json:"configMapRef,omitempty"`
	// +optional
	SecretRef *LocalObjectReference `json:"secretRef,omitempty"`
}

func (e EnvFromSource) ToKubernetesType() corev1.EnvFromSource {
	env := corev1.EnvFromSource{
		Prefix: e.Prefix,
	}
	if e.ConfigMapRef != nil {
		env.ConfigMapRef = ptr.To(corev1.ConfigMapEnvSource{
			LocalObjectReference: e.ConfigMapRef.ToKubernetesType(),
		})
	}
	if e.SecretRef != nil {
		env.SecretRef = ptr.To(corev1.SecretEnvSource{
			LocalObjectReference: e.SecretRef.ToKubernetesType(),
		})
	}
	return env
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#execaction-v1-core.
type ExecAction struct {
	// +optional
	// +listType=atomic
	Command []string `json:"command,omitempty"`
}

func (e ExecAction) ToKubernetesType() corev1.ExecAction {
	return corev1.ExecAction{
		Command: e.Command,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#httpgetaction-v1-core.
type HTTPGetAction struct {
	// +optional
	Path string             `json:"path,omitempty"`
	Port intstr.IntOrString `json:"port"`
	// +optional
	Host string `json:"host,omitempty"`
	// +optional
	Scheme corev1.URIScheme `json:"scheme,omitempty"`
}

func (e HTTPGetAction) ToKubernetesType() corev1.HTTPGetAction {
	return corev1.HTTPGetAction{
		Path:   e.Path,
		Port:   e.Port,
		Host:   e.Host,
		Scheme: e.Scheme,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core.
type ProbeHandler struct {
	// +optional
	Exec *ExecAction `json:"exec,omitempty"`
	// +optional
	HTTPGet *HTTPGetAction `json:"httpGet,omitempty"`
}

func (p ProbeHandler) ToKubernetesType() corev1.ProbeHandler {
	var probe corev1.ProbeHandler
	if p.Exec != nil {
		probe.Exec = ptr.To(p.Exec.ToKubernetesType())
	}
	if p.HTTPGet != nil {
		probe.HTTPGet = ptr.To(p.HTTPGet.ToKubernetesType())
	}
	return probe
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core.
type Probe struct {
	ProbeHandler `json:",inline"`
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
}

func (p Probe) ToKubernetesType() corev1.Probe {
	return corev1.Probe{
		ProbeHandler:        p.ProbeHandler.ToKubernetesType(),
		InitialDelaySeconds: p.InitialDelaySeconds,
		TimeoutSeconds:      p.TimeoutSeconds,
		PeriodSeconds:       p.PeriodSeconds,
		SuccessThreshold:    p.SuccessThreshold,
		FailureThreshold:    p.FailureThreshold,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core.
type ResourceRequirements struct {
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`
}

func (r ResourceRequirements) ToKubernetesType() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Limits:   r.Limits,
		Requests: r.Requests,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#securitycontext-v1-core.
type SecurityContext struct {
	// +optional
	Capabilities *corev1.Capabilities `json:"capabilities,omitempty"`
	// +optional
	Privileged *bool `json:"privileged,omitempty"`
	// +optional
	RunAsUser *int64 `json:"runAsUser,omitempty"`
	// +optional
	RunAsGroup *int64 `json:"runAsGroup,omitempty"`
	// +optional
	RunAsNonRoot *bool `json:"runAsNonRoot,omitempty"`
	// +optional
	ReadOnlyRootFilesystem *bool `json:"readOnlyRootFilesystem,omitempty"`
	// +optional
	AllowPrivilegeEscalation *bool `json:"allowPrivilegeEscalation,omitempty"`
}

func (s SecurityContext) ToKubernetesType() corev1.SecurityContext {
	return corev1.SecurityContext{
		Capabilities:             s.Capabilities,
		Privileged:               s.Privileged,
		RunAsUser:                s.RunAsUser,
		RunAsGroup:               s.RunAsGroup,
		RunAsNonRoot:             s.RunAsNonRoot,
		ReadOnlyRootFilesystem:   s.ReadOnlyRootFilesystem,
		AllowPrivilegeEscalation: s.AllowPrivilegeEscalation,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podsecuritycontext-v1-core
type PodSecurityContext struct {
	// +optional
	SELinuxOptions *corev1.SELinuxOptions `json:"seLinuxOptions,omitempty"`
	// +optional
	RunAsUser *int64 `json:"runAsUser,omitempty"`
	// +optional
	RunAsGroup *int64 `json:"runAsGroup,omitempty"`
	// +optional
	RunAsNonRoot *bool `json:"runAsNonRoot,omitempty"`
	// +optional
	// +listType=atomic
	SupplementalGroups []int64 `json:"supplementalGroups,omitempty"`
	// +optional
	FSGroup *int64 `json:"fsGroup,omitempty"`
	// +optional
	FSGroupChangePolicy *corev1.PodFSGroupChangePolicy `json:"fsGroupChangePolicy,omitempty"`
	// +optional
	SeccompProfile *corev1.SeccompProfile `json:"seccompProfile,omitempty"`
	// +optional
	AppArmorProfile *corev1.AppArmorProfile `json:"appArmorProfile,omitempty"`
}

func (s PodSecurityContext) ToKubernetesType() corev1.PodSecurityContext {
	return corev1.PodSecurityContext{
		SELinuxOptions:      s.SELinuxOptions,
		RunAsUser:           s.RunAsUser,
		RunAsGroup:          s.RunAsGroup,
		RunAsNonRoot:        s.RunAsNonRoot,
		SupplementalGroups:  s.SupplementalGroups,
		FSGroup:             s.FSGroup,
		FSGroupChangePolicy: s.FSGroupChangePolicy,
		SeccompProfile:      s.SeccompProfile,
		AppArmorProfile:     s.AppArmorProfile,
	}
}

// Refer to the Kubernetes docs: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#serviceport-v1-core
type ServicePort struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
}

func (r ServicePort) ToKubernetesType() corev1.ServicePort {
	return corev1.ServicePort{
		Name: r.Name,
		Port: r.Port,
	}
}
