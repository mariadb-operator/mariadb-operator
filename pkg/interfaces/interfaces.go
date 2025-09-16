package interfaces

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientpkg "sigs.k8s.io/controller-runtime/pkg/client"
)

type Imager interface {
	GetImagePullPolicy() corev1.PullPolicy
	GetImagePullSecrets() []mariadbv1alpha1.LocalObjectReference
	GetImage(env *environment.OperatorEnv) string
}

type TLSProvider interface {
	IsTLSEnabled() bool
	TLSCABundleSecretKeyRef() mariadbv1alpha1.SecretKeySelector
	TLSClientCertSecretKey() types.NamespacedName
	TLSServerCertSecretKey() types.NamespacedName
}

type Replicator interface {
	GetReplicas() int32
}

type Connector interface {
	GetHost() string
	GetPort() int32
	GetSUName() string
	GetSUCredential() *mariadbv1alpha1.SecretKeySelector
}

type GaleraProvider interface {
	IsGaleraEnabled() bool
}

type MariaDBObject interface {
	clientpkg.Object
	runtime.Object
	Connector
	GaleraProvider
	Imager
	Replicator
	TLSProvider

	IsReady() bool
}
