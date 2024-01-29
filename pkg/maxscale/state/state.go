package state

import "strings"

// See: https://mariadb.com/kb/en/mariadb-maxscale-25-mariadb-maxscale-configuration-guide/#server

func IsMaster(state string) bool {
	return strings.Contains(state, "Master") && !strings.Contains(state, "Relay Master")
}

func IsSlave(state string) bool {
	return strings.Contains(state, "Slave")
}

func IsReady(state string) bool {
	return IsMaster(state) || IsSlave(state)
}

func InMaintenance(state string) bool {
	return strings.Contains(state, "Maintenance")
}
