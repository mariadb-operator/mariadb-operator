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

func main() {
	cmd := exec.Command("docker", "network", "inspect", "kind")
	output, err := cmd.CombinedOutput()

	if err != nil {
		fmt.Printf("%v", err)
	}

	var network []Network
	json.Unmarshal([]byte(output), &network)

	configs := network[0].Ipam.Config

	var configIndex = -1
	var index int = 0

	for configIndex == -1 {
		if strings.HasPrefix(configs[index].Subnet, "172.") {
			configIndex = index
		} else {
			index++
		}
	}

	fmt.Print(configs[index].Subnet)
}
