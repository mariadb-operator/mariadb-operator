package replication

var (
	GaleraConfigVolume    = "galera"
	GaleraConfigMountPath = "/etc/mysql/mariadb.conf.d"

	GaleraClusterPortName = "cluster"
	GaleraClusterPort     = int32(4444)
	GaleraISTPortName     = "ist"
	GaleraISTPort         = int32(4567)
	GaleraSSTPortName     = "sst"
	GaleraSSTPort         = int32(4568)
	AgentPortName         = "agent"
)
