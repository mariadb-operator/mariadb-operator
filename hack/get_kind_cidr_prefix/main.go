package main

import (
	"fmt"
	"os"

	"github.com/mariadb-operator/mariadb-operator/pkg/docker"
)

func main() {
	prefix, err := docker.GetKindCidrPrefix()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Print(prefix)
}
