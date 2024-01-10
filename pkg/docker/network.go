package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Config struct {
	Subnet  string
	Gateway string
}

type IPAM struct {
	Config []Config
}

type Network struct {
	Ipam IPAM
}

func (ipam *IPAM) findConfigWithPrefix(prefix string) *Config {
	for _, config := range ipam.Config {
		if strings.HasPrefix(config.Subnet, prefix) {
			return &config
		}
	}
	return nil
}

func GetDockerCidr(network string) (string, error) {
	command := exec.Command("docker", "network", "inspect", network)
	output, err := command.Output()
	if err != nil {
		return "", errors.New("could not execute docker command")
	}

	var networks []Network
	err = json.Unmarshal([]byte(output), &networks)
	if err != nil {
		return "", errors.New("invalid json")
	}

	config := networks[0].Ipam.findConfigWithPrefix("172.")
	if config == nil {
		return "", errors.New("could not find config with prefix '172.'")
	}
	return config.Subnet, nil
}

func GetCidrPrefix(cidr string) string {
	ip := strings.Split(cidr, "/")
	parts := strings.Split(ip[0], ".")
	return fmt.Sprintf("%s.%s", parts[0], parts[1])
}

func GetDockerCidrPrefix(network string) (string, error) {
	cidr, err := GetDockerCidr(network)
	if err != nil {
		return "", err
	}
	return GetCidrPrefix(cidr), nil
}
