package interfaces

import (
	"context"
	"io"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v26/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v26/pkg/environment"

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
	IsTLSMutual() bool
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

type BlobStorage interface {
	PutObjectWithOptions(ctx context.Context, fileName string, reader io.Reader, size int64) error
	FPutObjectWithOptions(ctx context.Context, fileName string) error
	GetObjectWithOptions(ctx context.Context, fileName string) (io.ReadCloser, error)
	FGetObjectWithOptions(ctx context.Context, fileName string) error
	RemoveWithOptions(ctx context.Context, fileName string) error
	Exists(ctx context.Context, fileName string) (bool, error)
	PrefixedFileName(fileName string) string
	UnprefixedFilename(fileName string) string
	GetPrefix() string
	ListObjectsWithOptions(ctx context.Context) ([]string, error)
	IsAuthenticated(ctx context.Context) bool
	IsNotFound(err error) bool
}

// Cordonable means that a resource can be "cordoned" (external connections interrupted)
// Currently: `MaxScale` and `MariaDB`
type Cordonable interface {
	IsCordonEnabled() bool
}
