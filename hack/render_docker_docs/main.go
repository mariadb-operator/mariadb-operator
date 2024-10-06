package main

import (
	"log"
	"os"
	"text/template"
)

func main() {
	version := os.Getenv("VERSION")
	if version == "" {
		log.Println("Environment variable \"VERSION\" is mandatory")
		os.Exit(1)
	}

	tplBytes, err := os.ReadFile("docs/DOCKER.md.gotmpl")
	if err != nil {
		log.Printf("Error reading docs template: %v\n", err)
		os.Exit(1)
	}

	tpl := template.New("docker")
	tpl, err = tpl.Parse(string(tplBytes))
	if err != nil {
		log.Printf("Error parsing docs template: %v\n", err)
		os.Exit(1)
	}

	file, err := os.Create("docs/DOCKER.md")
	if err != nil {
		log.Printf("Error creating docs: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	err = tpl.Execute(file, struct {
		OperatorVersion string
	}{
		OperatorVersion: version,
	})
	if err != nil {
		log.Printf("Error rendering docs: %v\n", err)
		os.Exit(1)
	}
}
