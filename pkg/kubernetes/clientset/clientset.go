package kubeclientset

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type ClientSet struct {
	kubernetes.Clientset
}

func NewClientSet() (*ClientSet, error) {
	restConfig, err := restConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting REST config: %v", err)
	}
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating clientset: %v", err)
	}
	return &ClientSet{
		Clientset: *clientSet,
	}, nil
}

func (c *ClientSet) GetMariaDB(ctx context.Context, name, namespace string) (*mariadbv1alpha1.MariaDB, error) {
	path := fmt.Sprintf("/apis/mariadb.mmontes.io/v1alpha1/namespaces/%s/mariadbs/%s", namespace, name)
	bytes, err := c.RESTClient().Get().AbsPath(path).DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("error requesting '%s' MariaDB in namespace '%s': %v", name, namespace, err)
	}
	var mdb mariadbv1alpha1.MariaDB
	if err := json.Unmarshal(bytes, &mdb); err != nil {
		return nil, fmt.Errorf("error decoding MariaDB: %v", err)
	}
	return &mdb, nil
}

func restConfig() (*rest.Config, error) {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}
