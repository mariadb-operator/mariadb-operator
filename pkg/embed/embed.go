package embed

import "embed"

//go:embed mariadb-docker/* ssl/*
var fs embed.FS

func ReadEntrypoint() ([]byte, error) {
	return fs.ReadFile("mariadb-docker/docker-entrypoint.sh")
}

func ReadCACertsPEM() ([]byte, error) {
	return fs.ReadFile("ssl/ca.crt")
}
