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

type PrivateKey string

const (
	PrivateKeyTypeECDSA PrivateKey = "ecdsa"
	PrivateKeyTypeRSA   PrivateKey = "rsa"
)

func GeneratePrivateKey() (crypto.Signer, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("error generating ECDSA private key: %v", err)
	}
	return privateKey, nil
}

func MarshalPrivateKey(signer crypto.Signer) ([]byte, error) {
	privateKey, ok := signer.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("signer is not an ECDSA private key")
	}
	return x509.MarshalECPrivateKey(privateKey)
}

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
