package main

import (
	"fmt"

	docker "github.com/mariadb-operator/mariadb-operator/pkg/docker"
)

func main() {
	prefix, err := docker.GetKindCidrPrefix()

	if err == nil {
		fmt.Print(prefix)
	} else {
		fmt.Println(err)
	}
}
