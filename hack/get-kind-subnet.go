package main

import (
	"encoding/json"
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

func main() {
	cmd := exec.Command("docker", "network", "inspect", "kind")
	output, err := cmd.CombinedOutput()

	if err != nil {
		fmt.Printf("%v", err)
	}

	var network []Network
	json.Unmarshal([]byte(output), &network)

	config := network[0].Ipam.findConfigWithPrefix("172.")

	if config != nil {
		fmt.Print(config.Subnet)
	}
}
