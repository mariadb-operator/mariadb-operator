package pki

import (
	"path/filepath"

	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
)

const (
	PKIVolume           = "pki"
	MariadbPKIMountPath = "/etc/pki"
)

var (
	MariadbTLSCACertPath     = filepath.Join(MariadbPKIMountPath, pki.CACertKey)
	MariadbTLSServerCertPath = filepath.Join(MariadbPKIMountPath, "server.crt")
	MariadbTLSServerKeyPath  = filepath.Join(MariadbPKIMountPath, "server.key")
	MariadbTLSClientCertPath = filepath.Join(MariadbPKIMountPath, "client.crt")
	MariadbTLSClientKeyPath  = filepath.Join(MariadbPKIMountPath, "client.key")
)
