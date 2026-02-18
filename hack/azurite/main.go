package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var containers = []string{
	"physicalbackup",
	"test-physicalbackup",
	"binlogs",
}

const (
	secretName      = "azurite-certs"
	secretNamespace = "default"
	secretCertKey   = "cert.pem"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	k8sClient, err := getK8sClient()
	if err != nil {
		panic(err)
	}
	cert, err := getCertFromSecret(ctx, k8sClient)
	if err != nil {
		panic(err)
	}

	client, err := NewAzBlobClient(cert)
	if err != nil {
		panic(err)
	}

	for _, containerName := range containers {
		fmt.Printf("Creating container (%s)\n", containerName)
		if err = client.CreateContainerIfNotExists(ctx, containerName); err != nil {
			log.Fatal(err)
		}
	}
}

type AzBlobClient struct {
	Client *azblob.Client
}

func NewAzBlobClient(cert string) (*AzBlobClient, error) {
	serviceURL := os.Getenv("AZURE_SERVICE_URL")
	accountName := os.Getenv("AZURE_STORAGE_ACCOUNT_NAME")
	accountKey := os.Getenv("AZURE_STORAGE_ACCOUNT_KEY")

	if serviceURL == "" {
		log.Fatal("AZURE_SERVICE_URL is empty")
	}

	if accountName == "" {
		log.Fatal("AZURE_STORAGE_ACCOUNT_NAME is empty")
	}

	if accountKey == "" {
		log.Fatal("AZURE_STORAGE_ACCOUNT_KEY is empty")
	}

	clientOptions, err := getClientOptions(cert)
	if err != nil {
		return nil, err
	}

	cred, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, err
	}

	client, err := azblob.NewClientWithSharedKeyCredential(serviceURL, cred, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("error creating new client with default credentials: %w", err)
	}

	return &AzBlobClient{
		Client: client,
	}, nil
}

func (c *AzBlobClient) CreateContainerIfNotExists(ctx context.Context, containerName string) error {
	_, err := c.Client.CreateContainer(ctx, containerName, nil)
	if err != nil {
		// Ref: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azcore#ResponseError
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) {
			switch respErr.StatusCode {
			case http.StatusConflict:
				return nil // Already exists
			default:
				return respErr
			}
		}
	}

	return nil
}

func getClientOptions(cert string) (*azblob.ClientOptions, error) {
	transport, err := getTransport(cert)
	if err != nil {
		return nil, fmt.Errorf("error getting transport: %w", err)
	}

	return &azblob.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Transport: &http.Client{
				Transport: transport,
			},
		},
	}, nil
}

func getTransport(cert string) (http.RoundTripper, error) {
	caCertPool := x509.NewCertPool()

	if ok := caCertPool.AppendCertsFromPEM([]byte(cert)); !ok {
		return nil, errors.New("unable to add CA cert to pool")
	}

	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			RootCAs:            caCertPool,
			InsecureSkipVerify: false,
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}, nil
}

func getK8sClient() (client.Client, error) {
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting REST config: %v", err)
	}
	k8sClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("error creating Kubernetes client: %v", err)
	}
	return k8sClient, nil
}

func getCertFromSecret(ctx context.Context, client client.Client) (string, error) {
	key := types.NamespacedName{
		Name:      secretName,
		Namespace: secretNamespace,
	}
	var secret corev1.Secret
	if err := client.Get(ctx, key, &secret); err != nil {
		return "", err
	}

	data, ok := secret.Data[secretCertKey]
	if !ok {
		return "", fmt.Errorf("secret key \"%s\" not found", secretCertKey)
	}
	return string(data), nil
}
