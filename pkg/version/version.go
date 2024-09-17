package version

import (
	"fmt"

	"github.com/hashicorp/go-version"
	"github.com/mariadb-operator/mariadb-operator/pkg/docker"
)

func GetMinorVersion(image string) (string, error) {
	tag, err := docker.GetTag(image)
	if err != nil {
		return "", fmt.Errorf("invalid image: %v", err)
	}

	v, err := version.NewSemver(tag)
	if err != nil {
		return "", fmt.Errorf("error parsing version: %v", err)
	}
	segments := v.Segments()

	return fmt.Sprintf("%d.%d", segments[0], segments[1]), nil
}
