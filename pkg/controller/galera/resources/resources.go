package replication

var (
	GaleraConfigVolume    = "galera"
	GaleraConfigMountPath = "/etc/mysql/mariadb.conf.d"

	AgentAuthVolume      = "agent-auth"
	AgentAuthVolumeMount = "/var/run/secrets/mariadb-operator/agent"

	GaleraSSTPortName     = "sst"
	GaleraSSTPort         = int32(4444)
	GaleraClusterPortName = "cluster"
	GaleraClusterPort     = int32(4567)
	GaleraISTPortName     = "ist"
	GaleraISTPort         = int32(4568)
	AgentPortName         = "agent"
)
