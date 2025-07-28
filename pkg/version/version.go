package version

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/docker"
)

// Option represents a function that applies a configuration to a Version instance.
type Option func(opts *Options)

// WithDefaultVersion sets a default version if parsing the Docker image tag fails.
func WithDefaultVersion(defaultVersion string) Option {
	return func(opts *Options) {
		opts.defaultVersion = defaultVersion
	}
}

// WithLogger sets a logger for the Version instance.
func WithLogger(logger logr.Logger) Option {
	return func(opts *Options) {
		opts.logger = logger
	}
}

// Options to be used with Version.
type Options struct {
	defaultVersion string
	logger         logr.Logger
}

// Version wraps a HashiCorp version.Version struct to provide additional functionalities for better convenience.
type Version struct {
	innerVersion version.Version
}

// GetMinorVersion extracts and returns the "major.minor" part of the version.
func (v *Version) GetMinorVersion() (string, error) {
	segments := v.innerVersion.Segments()
	if len(segments) < 2 {
		return "", fmt.Errorf("invalid version: %v", v.innerVersion.String())
	}
	return fmt.Sprintf("%d.%d", segments[0], segments[1]), nil
}

// Compare compares the current version with another semantic version string.
func (v *Version) Compare(other string) (int, error) {
	otherVersion, err := version.NewSemver(other)
	if err != nil {
		return 0, fmt.Errorf("error parsing version '%s': %v", other, err)
	}
	return v.innerVersion.Compare(otherVersion), nil
}

// GreaterThanOrEqual checks if the current version is greater than or equal to another version.
func (v *Version) GreaterThanOrEqual(other string) (bool, error) {
	result, err := v.Compare(other)
	if err != nil {
		return false, fmt.Errorf("error comparing versions: %v", err)
	}
	return result >= 0, nil
}

// NewVersion constructs a new Version instance from a given Docker image tag.
func NewVersion(image string, vOpts ...Option) (*Version, error) {
	opts := Options{
		logger: logr.Discard(),
	}
	for _, opt := range vOpts {
		opt(&opts)
	}

	var versions []string
	tag, err := docker.GetTag(image)
	if err != nil {
		opts.logger.Error(err, "unable to parse tag from image", "image", image)
	} else {
		versions = append(versions, tag)
	}
	if opts.defaultVersion != "" {
		opts.logger.V(1).Info("configuring default version", "version", opts.defaultVersion)
		versions = append(versions, opts.defaultVersion)
	}

	for _, v := range versions {
		innerVersion, err := version.NewSemver(v)
		if err == nil {
			return &Version{innerVersion: *innerVersion}, nil
		}
		opts.logger.Error(err, "unable to parse version", "version", v)
	}
	return nil, fmt.Errorf("unable to parse version from image \"%s\" nor default image", image)
}
