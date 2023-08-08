package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
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
	fmt.Println(network[0].Ipam.Config[0].Subnet)
}
