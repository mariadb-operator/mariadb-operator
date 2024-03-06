package embed

import (
	"embed"
	_ "embed"
)

//go:embed mariadb-docker/*
var fs embed.FS

func ReadEntrypoint() ([]byte, error) {
	return fs.ReadFile("mariadb-docker/docker-entrypoint.sh")
}
