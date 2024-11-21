package pki

import (
	"path/filepath"

	"github.com/mariadb-operator/mariadb-operator/pkg/pki"
)

const (
	PKIVolume    = "pki"
	PKIMountPath = "/etc/pki"

	ServerCertKey = "server.crt"
	ServerKeyKey  = "server.key"

	ClientCertKey = "client.crt"
	ClientKeyKey  = "client.key"

	AdminCertKey = "admin.crt"
	AdminKeyKey  = "admin.key"
)

var (
	CACertPath = filepath.Join(PKIMountPath, pki.CACertKey)

	ServerCertPath = filepath.Join(PKIMountPath, ServerCertKey)
	ServerKeyPath  = filepath.Join(PKIMountPath, ServerKeyKey)

	ClientCertPath = filepath.Join(PKIMountPath, ClientCertKey)
	ClientKeyPath  = filepath.Join(PKIMountPath, ClientKeyKey)

	AdminCertPath = filepath.Join(PKIMountPath, AdminCertKey)
	AdminKeyPath  = filepath.Join(PKIMountPath, AdminKeyKey)
)
