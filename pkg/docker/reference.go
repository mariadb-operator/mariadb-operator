package docker

import (
	"errors"
	"fmt"

	"github.com/distribution/reference"
)

func GetTag(image string) (string, error) {
	ref, err := reference.Parse(image)
	if err != nil {
		return "", fmt.Errorf("error parsing reference: %v", err)
	}

	if tagged, ok := ref.(reference.NamedTagged); ok {
		return tagged.Tag(), nil
	}
	return "", errors.New("image does not have a tag")
}

func SetTagOrDigest(sourceImage, targetImage string) (string, error) {
	sourceRef, err := reference.Parse(sourceImage)
	if err != nil {
		return "", fmt.Errorf("error parsing source reference: %v", err)
	}

	targetRef, err := reference.ParseNamed(targetImage)
	if err != nil {
		return "", fmt.Errorf("error parsing target reference: %v", err)
	}
	targetRef = reference.TrimNamed(targetRef)

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
