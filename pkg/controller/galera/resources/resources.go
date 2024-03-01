package replication

var (
	GaleraConfigVolume    = "galera"
	GaleraConfigMountPath = "/etc/mysql/mariadb.conf.d"

	GaleraInitConfigVolume = "galera-init"
	GaleraInitConfigPath   = "/init"
	GaleraInitConfigKey    = "entrypoint.sh"

	GaleraClusterPortName = "cluster"
	GaleraClusterPort     = int32(4444)
	GaleraISTPortName     = "ist"
	GaleraISTPort         = int32(4567)
	GaleraSSTPortName     = "sst"
	GaleraSSTPort         = int32(4568)
	AgentPortName         = "agent"
)
