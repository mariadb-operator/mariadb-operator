package metadata

var (
	WatchLabel                            = "k8s.mariadb.com/watch"
	PhysicalBackupNameLabel               = "physicalbackup.k8s.mariadb.com/name"
	KubernetesServiceLabel                = "kubernetes.io/service-name"
	KubernetesEndpointSliceManagedByLabel = "endpointslice.kubernetes.io/managed-by"
	KubernetesEndpointSliceManagedByValue = "mariadb-operator.k8s.mariadb.com"

	ReplicationAnnotation = "k8s.mariadb.com/replication"
	GtidAnnotation        = "k8s.mariadb.com/gtid"
	GaleraAnnotation      = "k8s.mariadb.com/galera"
	MariadbAnnotation     = "k8s.mariadb.com/mariadb"

	ConfigAnnotation       = "k8s.mariadb.com/config"
	ConfigTLSAnnotation    = "k8s.mariadb.com/config-tls"
	ConfigGaleraAnnotation = "k8s.mariadb.com/config-galera"

	TLSCAAnnotation           = "k8s.mariadb.com/ca"
	TLSServerCertAnnotation   = "k8s.mariadb.com/server-cert"
	TLSClientCertAnnotation   = "k8s.mariadb.com/client-cert"
	TLSAdminCertAnnotation    = "k8s.mariadb.com/admin-cert"
	TLSListenerCertAnnotation = "k8s.mariadb.com/listener-cert"

	WebhookConfigAnnotation = "k8s.mariadb.com/webhook"

	MetaCtrlFieldPath = ".metadata.controller"
)
