package example_client

import (
	"context"
	"fmt"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/mariadb/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func main() {
	// get in -cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// use the clientset to list all database resources in the cluster
	// the clientset also supports all other top-level mariadb resources, such as User
	dbs, err := clientset.MariadbV1alpha1().Databases("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	for _, db := range dbs.Items {
		fmt.Printf("Found db %s in namespace %s with spec %+v\n", db.GetName(), db.GetNamespace(), db.Spec)
	}

	// use the clientset to create a new user in default namespace
	// the clientset also supports all other top-level mariadb resources
	user, err := clientset.MariadbV1alpha1().Users("default").Create(context.Background(), &mariadbv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "example-user",
		},
		Spec: mariadbv1alpha1.UserSpec{
			MariaDBRef: mariadbv1alpha1.MariaDBRef{
				ObjectReference: mariadbv1alpha1.ObjectReference{
					Name:      "mariadb",
					Namespace: "default",
				},
			},
			PasswordSecretKeyRef: &mariadbv1alpha1.SecretKeySelector{
				LocalObjectReference: mariadbv1alpha1.LocalObjectReference{
					Name: "mariadb-user-secret",
				},
				Key: "password",
			},
			Host: "%",
		},
	}, metav1.CreateOptions{})
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("created user %+v\n", user)
}
