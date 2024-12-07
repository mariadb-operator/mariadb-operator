package metadata

var (
	WatchLabel = "k8s.mariadb.com/watch"

	ReplicationAnnotation = "k8s.mariadb.com/replication"
	GaleraAnnotation      = "k8s.mariadb.com/galera"
	MariadbAnnotation     = "k8s.mariadb.com/mariadb"

	ConfigAnnotation       = "k8s.mariadb.com/config"
	ConfigGaleraAnnotation = "k8s.mariadb.com/config-galera"

	TLSCAAnnotation         = "k8s.mariadb.com/ca"
	TLSServerCertAnnotation = "k8s.mariadb.com/server-cert"
	TLSClientCertAnnotation = "k8s.mariadb.com/client-cert"

	WebhookConfigAnnotation = "k8s.mariadb.com/webhook"
)
