package pki

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	tlsCert              = "tls.crt"
	tlsKey               = "tls.key"
	lookaheadInterval    = 90 * 24 * time.Hour
	certValidityDuration = 10 * 365 * 24 * time.Hour
)

type KeyPair struct {
	Cert    *x509.Certificate
	Key     *rsa.PrivateKey
	CertPEM []byte
	KeyPEM  []byte
}

func (k *KeyPair) IsValid() bool {
	return k.Cert != nil && k.Key != nil && len(k.CertPEM) > 0 && len(k.KeyPEM) > 0
}

func (k *KeyPair) FillTLSSecret(secret *corev1.Secret) {
	secret.Data[tlsCert] = k.CertPEM
	secret.Data[tlsKey] = k.KeyPEM
}

func KeyPairFromTLSSecret(secret *corev1.Secret) (*KeyPair, error) {
	if secret.Data == nil || len(secret.Data[tlsCert]) == 0 || len(secret.Data[tlsKey]) == 0 {
		return nil, errors.New("TLS Secret is empty")
	}
	certPEM := secret.Data[tlsCert]
	keyPEM := secret.Data[tlsKey]

	certDer, _ := pem.Decode(certPEM)
	if certDer == nil {
		return nil, errors.New("Bad certificate")
	}
	cert, err := x509.ParseCertificate(certDer.Bytes)
	if err != nil {
		return nil, fmt.Errorf("Error parsing x509 certificate: %v", err)
	}
	keyDer, _ := pem.Decode(keyPEM)
	if keyDer == nil {
		return nil, fmt.Errorf("Bad private key")
	}
	key, err := x509.ParsePKCS1PrivateKey(keyDer.Bytes)
	if err != nil {
		return nil, fmt.Errorf("Error parsing PKCS1 private key: %v", err)
	}
	return &KeyPair{
		Cert:    cert,
		Key:     key,
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	}, nil
}

type CAOpts struct {
	CommonName   string
	Organization string
}

type CAOpt func(*CAOpts)

func WithCACommonName(name string) CAOpt {
	return func(c *CAOpts) {
		c.CommonName = name
	}
}

func WithCAOrganization(org string) CAOpt {
	return func(c *CAOpts) {
		c.Organization = org
	}
}

func CreateCACert(begin, end time.Time, opts ...CAOpt) (*KeyPair, error) {
	caOpts := CAOpts{
		CommonName:   "mariadb-operator",
		Organization: "mariadb-operator",
	}
	for _, setOpt := range opts {
		setOpt(&caOpts)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		Subject: pkix.Name{
			CommonName:   caOpts.CommonName,
			Organization: []string{caOpts.Organization},
		},
		DNSNames: []string{
			caOpts.CommonName,
		},
		NotBefore:             begin,
		NotAfter:              end,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	return createKeyPair(tpl, nil)
}

func CreateCert(caKeyPair *KeyPair, begin, end time.Time, commonName string, dnsNames []string) (*KeyPair, error) {
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		DNSNames:              dnsNames,
		NotBefore:             begin,
		NotAfter:              end,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	return createKeyPair(tpl, caKeyPair)
}

func ValidCert(caKeyPair *KeyPair, certKeyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
	if !caKeyPair.IsValid() {
		return false, errors.New("Invalid CA KeyPair")
	}
	if !certKeyPair.IsValid() {
		return false, errors.New("Invalid certificate KeyPair")
	}

	pool := x509.NewCertPool()
	pool.AddCert(caKeyPair.Cert)

	_, err := tls.X509KeyPair(certKeyPair.CertPEM, certKeyPair.KeyPEM)
	if err != nil {
		return false, err
	}

	certBytes, _ := pem.Decode(certKeyPair.CertPEM)
	if certBytes == nil {
		return false, err
	}

	parsedCert, err := x509.ParseCertificate(certBytes.Bytes)
	if err != nil {
		return false, err
	}
	_, err = parsedCert.Verify(x509.VerifyOptions{
		DNSName:     dnsName,
		Roots:       pool,
		CurrentTime: at,
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func ValidCACert(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
	return ValidCert(keyPair, keyPair, dnsName, at)
}

func CreateOrUpdateTLSSecret(ctx context.Context, client client.Client, key types.NamespacedName, keyPair *KeyPair) error {
	var secret corev1.Secret
	if err := client.Get(ctx, key, &secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("Error getting TLS secret: %v", err)
		}
		emptySecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Data: nil,
		}
		if err := client.Create(ctx, &emptySecret); err != nil {
			return fmt.Errorf("Error creating Secret: %v", err)
		}
		if err := client.Get(ctx, key, &secret); err != nil {
			return fmt.Errorf("Error getting Secret: %v", err)
		}
	}

	keyPair.FillTLSSecret(&secret)
	if err := client.Update(ctx, &secret); err != nil {
		return fmt.Errorf("Error updating Secret: %v", err)
	}
	return nil
}

type RefreshResult struct {
	RefreshedCA   bool
	RefreshedCert bool
}

func RefreshCert(ctx context.Context, client client.Client, caKey, certKey types.NamespacedName,
	caName, certName string) (*RefreshResult, error) {
	refreshResult := &RefreshResult{}
	var caSecret corev1.Secret
	if err := client.Get(ctx, caKey, &caSecret); err != nil {
		return refreshResult, fmt.Errorf("Error getting CA Secret: %v", err)
	}
	var caKeyPair *KeyPair
	var err error
	caKeyPair, err = KeyPairFromTLSSecret(&caSecret)
	if err != nil {
		return refreshResult, fmt.Errorf("Error getting CA KeyPair: %v", err)
	}

	valid, err := ValidCACert(caKeyPair, caName, lookaheadTime())
	if caSecret.Data == nil || !valid || err != nil {
		begin := time.Now().Add(-1 * time.Hour)
		end := time.Now().Add(certValidityDuration)
		caKeyPair, err = CreateCACert(begin, end)
		if err != nil {
			return refreshResult, fmt.Errorf("Error creating CA cert: %v", err)
		}
		refreshResult.RefreshedCA = true
	}

	var certSecret corev1.Secret
	if err := client.Get(ctx, certKey, &certSecret); err != nil {
		return refreshResult, fmt.Errorf("Error getting certificate Secret: %v", err)
	}
	certKeyPair, err := KeyPairFromTLSSecret(&certSecret)
	if err != nil {
		return refreshResult, fmt.Errorf("Error getting certificate KeyPair: %v", err)
	}

	valid, err = ValidCert(caKeyPair, certKeyPair, certName, lookaheadTime())
	if refreshResult.RefreshedCA || certSecret.Data == nil || !valid || err != nil {
		begin := time.Now().Add(-1 * time.Hour)
		end := time.Now().Add(certValidityDuration)
		certKeyPair, err := CreateCert(caKeyPair, begin, end, certKeyPair.Cert.Subject.CommonName, certKeyPair.Cert.DNSNames)
		if err != nil {
			return refreshResult, fmt.Errorf("Error creating certificate %v", err)
		}
		if err := CreateOrUpdateTLSSecret(ctx, client, certKey, certKeyPair); err != nil {
			return refreshResult, fmt.Errorf("Error creating certificate TLS Secret: %v", err)
		}
		refreshResult.RefreshedCert = true
	}
	return refreshResult, nil
}

func lookaheadTime() time.Time {
	return time.Now().Add(lookaheadInterval)
}

func createKeyPair(tpl *x509.Certificate, caKeyPair *KeyPair) (*KeyPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	issuerCert := tpl
	if caKeyPair != nil {
		issuerCert = caKeyPair.Cert
	}
	issuerPrivateKey := key
	if caKeyPair != nil {
		issuerPrivateKey = caKeyPair.Key
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, issuerCert, key.Public(), issuerPrivateKey)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	certPEM, keyPEM, err := pemEncodeKeyPair(der, key)
	if err != nil {
		return nil, err
	}
	return &KeyPair{
		Cert:    cert,
		Key:     key,
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	}, nil
}

func pemEncodeKeyPair(certificateDER []byte, key *rsa.PrivateKey) (certPEM []byte, keyPEM []byte, err error) {
	certBuf := &bytes.Buffer{}
	if err := pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}); err != nil {
		return nil, nil, err
	}
	keyBuf := &bytes.Buffer{}
	if err := pem.Encode(keyBuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return nil, nil, err
	}
	return certBuf.Bytes(), keyBuf.Bytes(), nil
}
