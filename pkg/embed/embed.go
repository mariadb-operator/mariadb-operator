package embed

import (
	"context"
	"embed"
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	env "github.com/mariadb-operator/mariadb-operator/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//go:embed mariadb-docker/*
var fs embed.FS

func ReadEntrypoint(ctx context.Context, mariadb *mariadbv1alpha1.MariaDB, operatorEnv *env.OperatorEnv) ([]byte, error) {
	var minorVersion string
	var err error
	image := mariadb.Spec.Image
	logger := log.FromContext(ctx).
		WithName("entrypoint").
		WithValues("image", image).
		V(1)

	minorVersion, err = version.GetMinorVersion(image)
	if err != nil {
		logger.Info(
			"error getting entrypoint version. Using default version",
			"version", operatorEnv.MariadbEntrypointVersion,
			"err", err,
		)
		minorVersion = operatorEnv.MariadbEntrypointVersion
	}
	logger = logger.WithValues("version", minorVersion)

	bytes, err := readEntrypoint(minorVersion, logger)
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			bytes, err := readEntrypoint(operatorEnv.MariadbEntrypointVersion, logger)
			if err != nil {
				return nil, fmt.Errorf("error reading MariaDB default entrypoint: %v", err)
			}
			return bytes, nil
		}
		return nil, fmt.Errorf("error reading MariaDB entrypoint: %v", err)
	}
	return bytes, nil
}

func readEntrypoint(version string, logger logr.Logger) ([]byte, error) {
	entrypointPath := filepath.Join("mariadb-docker", version, "docker-entrypoint.sh")
	logger.Info("reading MariaDB entrypoint", "entrypoint", entrypointPath)
	return fs.ReadFile(entrypointPath)
}
