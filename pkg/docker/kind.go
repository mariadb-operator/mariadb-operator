package docker

func GetKindCidrPrefix() (string, error) {
	prefix, err := GetDockerCidrPrefix("kind")
	if err != nil {
		return "", err
	}
	return prefix, nil
}
