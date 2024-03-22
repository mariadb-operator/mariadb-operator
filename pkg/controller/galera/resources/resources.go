package replication

var (
	GaleraConfigVolume    = "galera"
	GaleraConfigMountPath = "/etc/mysql/mariadb.conf.d"

	GaleraSSTPortName     = "sst"
	GaleraSSTPort         = int32(4444)
	GaleraClusterPortName = "cluster"
	GaleraClusterPort     = int32(4567)
	GaleraISTPortName     = "ist"
	GaleraISTPort         = int32(4568)
	AgentPortName         = "agent"
)
