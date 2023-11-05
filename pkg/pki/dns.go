package pki

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/types"
)

type DNSNames struct {
	CommonName string
	Names      []string
}

func ServiceDNSNames(serviceKey types.NamespacedName) *DNSNames {
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "cluster.local"
	}
	return &DNSNames{
		CommonName: fmt.Sprintf("%s.%s.svc", serviceKey.Name, serviceKey.Namespace),
		Names: []string{
			fmt.Sprintf("%s.%s.svc.%s", serviceKey.Name, serviceKey.Namespace, clusterName),
			fmt.Sprintf("%s.%s", serviceKey.Name, serviceKey.Namespace),
			serviceKey.Name,
		},
	}
}
