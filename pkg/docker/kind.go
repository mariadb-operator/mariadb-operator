package docker

import "context"

func GetKindCidrPrefix(ctx context.Context) (string, error) {
	prefix, err := GetDockerCidrPrefix(ctx, "kind")
	if err != nil {
		return "", err
	}
	return prefix, nil
}
