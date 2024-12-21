package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	ds "github.com/mariadb-operator/mariadb-operator/pkg/datastructures"
)

// PrivateKey represents a type of private key.
type PrivateKey string

const (
	// PrivateKeyTypeECDSA represents an ECDSA private key.
	PrivateKeyTypeECDSA PrivateKey = "ecdsa"
	// PrivateKeyTypeRSA represents an RSA private key.
	PrivateKeyTypeRSA PrivateKey = "rsa"
)

// GeneratePrivateKey generates a new ECDSA private key.
func GeneratePrivateKey() (crypto.Signer, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("error generating ECDSA private key: %v", err)
	}
	return privateKey, nil
}

// MarshalPrivateKey marshals the given ECDSA private key to bytes.
func MarshalPrivateKey(signer crypto.Signer) ([]byte, error) {
	privateKey, ok := signer.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("signer is not an ECDSA private key")
	}
	return x509.MarshalECPrivateKey(privateKey)
}

// ParsePrivateKey parses a private key from the given bytes.
func ParsePrivateKey(bytes []byte, supportedKeys []PrivateKey) (crypto.Signer, error) {
	block, _ := pem.Decode(bytes) // private key should only have a single block
	if block == nil {
		return nil, errors.New("error decoding PEM")
	}
	if !isSupportedPrivateKeyBlock(block.Type, supportedKeys) {
		return nil, fmt.Errorf("unsupported PEM block: %v", block.Type)
	}

	switch block.Type {
	case pemBlockECPrivateKey:
		return x509.ParseECPrivateKey(block.Bytes)
	case pemBlockRSAPrivateKey:
		return x509.ParsePKCS1PrivateKey(block.Bytes) // backwards compatibility with webhook certs from previous versions
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %v", block.Type)
	}
}

func isSupportedPrivateKeyBlock(block string, supportedKeys []PrivateKey) bool {
	idx := ds.NewIndex(supportedKeys, func(pk PrivateKey) string {
		return string(pk)
	})
	if block == pemBlockECPrivateKey && ds.Has(idx, string(PrivateKeyTypeECDSA)) {
		return true
	}
	if block == pemBlockRSAPrivateKey && ds.Has(idx, string(PrivateKeyTypeRSA)) {
		return true
	}
	return false
}
