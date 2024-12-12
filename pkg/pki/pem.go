package pki

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
)

// PEMBlockType is a custom type representing the type of a PEM block.
type PEMBlockType string

const (
	// CertificatePEMBlockType represents the "CERTIFICATE" type for a PEM block.
	CertificatePEMBlockType PEMBlockType = "CERTIFICATE"
)

// BundleCertificatePEMs bundles multiple PEM-encoded certificate slices into a single bundle.
func BundleCertificatePEMs(logger logr.Logger, pems ...[]byte) ([]byte, error) {
	var bundle []byte
	var err error
	existingCerts := make(map[string]struct{})

	for _, pem := range pems {
		bundle, err = appendPEM(bundle, pem, existingCerts, logger)
		if err != nil {
			return nil, fmt.Errorf("error appending PEM: %v", err)
		}
	}
	if bundle == nil {
		return nil, errors.New("No certificate PEMs were found")
	}
	return bundle, nil
}

func appendPEM(bundle []byte, pemBytes []byte, existingCerts map[string]struct{}, logger logr.Logger) ([]byte, error) {
	var block *pem.Block
	for len(pemBytes) > 0 {
		block, pemBytes = pem.Decode(pemBytes)
		if block == nil {
			logger.Error(errors.New("Invalid PEM block"), "Error decoding PEM block. Ignoring...")
			break
		}
		if block.Type != string(CertificatePEMBlockType) {
			return nil, fmt.Errorf("invalid PEM certificate block, got block type: %v", block.Type)
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("invalid certificate in PEM block: %v", err)
		}
		certID := getCertID(cert)

		if _, ok := existingCerts[certID]; ok {
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
