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
	image := mariadb.Spec.Image
	logger := log.FromContext(ctx).
		WithName("entrypoint").
		WithValues("image", image).
		V(1)

	vOpts := []version.Option{
		version.WithLogger(logger),
	}
	if operatorEnv != nil && operatorEnv.MariadbDefaultVersion != "" {
		vOpts = append(vOpts, version.WithDefaultVersion(operatorEnv.MariadbDefaultVersion))
	}

	version, err := version.NewVersion(image, vOpts...)
	if err != nil {
		return nil, fmt.Errorf("error parsing version: %v", err)
	}
	minorVersion, err := version.GetMinorVersion()
	if err != nil {
		return nil, fmt.Errorf("error getting minor version: %v", err)
	}
	logger = logger.WithValues("version", minorVersion)

	bytes, err := readEntrypoint(minorVersion, logger)
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			bytes, err := readEntrypoint(operatorEnv.MariadbDefaultVersion, logger)
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
