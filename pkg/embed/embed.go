package embed

import "embed"

//go:embed mariadb-docker/*
var fs embed.FS

func ReadEntrypoint() ([]byte, error) {
	return fs.ReadFile("mariadb-docker/docker-entrypoint.sh")
}
