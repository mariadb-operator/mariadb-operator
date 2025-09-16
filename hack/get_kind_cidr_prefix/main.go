package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/docker"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prefix, err := docker.GetKindCidrPrefix(ctx)
	if err != nil {
		fmt.Println(err)
		cancel()
		os.Exit(1)
	}
	fmt.Print(prefix)
}
