package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
)

const (
	pemBlockCertificate   = "CERTIFICATE"
	pemBlockECPrivateKey  = "EC PRIVATE KEY"
	pemBlockRSAPrivateKey = "RSA PRIVATE KEY"
)

// BundleOption represents a function that applies a bundle configuration.
type BundleOption func(opts *BundleOptions)

// WithLogger sets the logger option.
func WithLogger(logger logr.Logger) BundleOption {
	return func(opts *BundleOptions) {
		opts.logger = logger
	}
}

// WithSkipExpired sets an option to skip expired certs.
func WithSkipExpired(skipExpired bool) BundleOption {
	return func(opts *BundleOptions) {
		opts.skipExpired = skipExpired
	}
}

// BundleOptions represents options for bundling certificates.
type BundleOptions struct {
	logger      logr.Logger
	skipExpired bool
}

// BundleCertificatePEMs bundles multiple PEM-encoded certificate slices into a single bundle.
func BundleCertificatePEMs(pems [][]byte, bundleOpts ...BundleOption) ([]byte, error) {
	opts := BundleOptions{
		logger:      logr.Discard(),
		skipExpired: false,
	}
	for _, opt := range bundleOpts {
		opt(&opts)
	}

	var bundle []byte
	var err error
	existingCerts := make(map[string]struct{})

	for _, pem := range pems {
		bundle, err = appendPEM(bundle, pem, existingCerts, opts)
		if err != nil {
			return nil, fmt.Errorf("error appending PEM: %v", err)
		}
	}
	if bundle == nil {
		return nil, errors.New("no certificate PEMs were found")
	}
	return bundle, nil
}

func appendPEM(bundle []byte, pemBytes []byte, existingCerts map[string]struct{}, opts BundleOptions) ([]byte, error) {
	var block *pem.Block
	for len(pemBytes) > 0 {
		block, pemBytes = pem.Decode(pemBytes)
		if block == nil {
			opts.logger.Error(errors.New("invalid PEM block"), "Error decoding PEM block. Ignoring...")
			break
		}
		if block.Type != string(pemBlockCertificate) {
			return nil, fmt.Errorf("invalid PEM certificate block, got block type: %v", block.Type)
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("invalid certificate in PEM block: %v", err)
		}
		certID := getCertID(cert)

		if _, ok := existingCerts[certID]; ok {
			opts.logger.V(1).Info("skipping existing certificate", "cert-id", certID)
			continue
		}

		now := time.Now()
		isExpired := now.Before(cert.NotBefore) || now.After(cert.NotAfter)
		if opts.skipExpired && isExpired {
			opts.logger.Info("skipping expired certificate", "cert-id", certID, "not-before", cert.NotBefore, "not-after", cert.NotAfter)
			continue
		}

		existingCerts[certID] = struct{}{}
		bundle = append(bundle, pem.EncodeToMemory(block)...)
	}
	return bundle, nil
}

func getCertID(cert *x509.Certificate) string {
	if cert.SerialNumber != nil {
		return fmt.Sprintf("%s-%s", cert.Subject.CommonName, cert.SerialNumber)
	}
	return cert.Subject.CommonName
}

func pemEncodeCertificate(bytes []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: pemBlockCertificate, Bytes: bytes})
}

func pemEncodePrivateKey(bytes []byte, signer crypto.Signer) ([]byte, error) {
	var blockType string
	switch signer.(type) {
	case *ecdsa.PrivateKey:
		blockType = pemBlockECPrivateKey
	case *rsa.PrivateKey:
		blockType = pemBlockRSAPrivateKey
	default:
		return nil, fmt.Errorf("unsupported private key: %t", signer)
	}
	return pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: bytes}), nil
}
