package main

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

func GetKindCidrPrefix() (string, error) {
	cmd := exec.Command("docker", "network", "inspect", "kind")
	output, err := cmd.Output()

	if err == nil {
		var network []Network
		json.Unmarshal([]byte(output), &network)

		config := network[0].Ipam.findConfigWithPrefix("172.")

		if config != nil {
			return config.Subnet, nil
		} else {
			return "", nil
		}
	} else {
		return "", errors.New("could not execute docker command")
	}
}

func main() {
	prefix, err := GetKindCidrPrefix()

	if err == nil {
		fmt.Print(prefix)
	} else {
		fmt.Println(err)
	}
}
