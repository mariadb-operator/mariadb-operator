package docker

import (
	"fmt"

	"github.com/distribution/reference"
)

func SetTagOrDigest(sourceImage, targetImage string) (string, error) {
	sourceRef, err := reference.Parse(sourceImage)
	if err != nil {
		return "", fmt.Errorf("error parsing source reference: %v", err)
	}

	targetRef, err := reference.ParseNamed(targetImage)
	if err != nil {
		return "", fmt.Errorf("error parsing target reference: %v", err)
	}

	var ref reference.Reference
	if tagged, ok := sourceRef.(reference.NamedTagged); ok {
		ref, err = reference.WithTag(targetRef, tagged.Tag())
		if err != nil {
			return "", fmt.Errorf("error setting tag: %v", err)
		}
	} else if digested, ok := sourceRef.(reference.Digested); ok {
		ref, err = reference.WithDigest(targetRef, digested.Digest())
		if err != nil {
			return "", fmt.Errorf("error setting digest: %v", err)
		}
	} else {
		return "", fmt.Errorf("source image \"%s\" does not have tag nor digest", sourceImage)
	}

	return ref.String(), nil
}
