package docker

func GetKindCidrPrefix() (string, error) {
	prefix, err := GetDockerCidrPrefix("kind")

	if err == nil {
		return prefix, nil
	} else {
		return "", err
	}
}
