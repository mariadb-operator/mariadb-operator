package version

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-version"
)

func GetMinorVersion(image string) (string, error) {
	parts := strings.SplitN(image, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid image format: %s", image)
	}

	v, err := version.NewSemver(parts[1])
	if err != nil {
		return "", fmt.Errorf("error parsing version: %v", err)
	}
	segments := v.Segments()

	return fmt.Sprintf("%d.%d", segments[0], segments[1]), nil
}
