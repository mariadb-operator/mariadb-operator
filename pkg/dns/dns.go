package dns

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/types"
)

type DNSNames struct {
	FQDN  string
	Names []string
}

func ServiceDNSNames(serviceKey types.NamespacedName) *DNSNames {
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "cluster.local"
	}
	fqdn := fmt.Sprintf("%s.%s.svc.%s", serviceKey.Name, serviceKey.Namespace, clusterName)
	return &DNSNames{
		FQDN: fqdn,
		Names: []string{
			fqdn,
			fmt.Sprintf("%s.%s", serviceKey.Name, serviceKey.Namespace),
			serviceKey.Name,
		},
	}
}
