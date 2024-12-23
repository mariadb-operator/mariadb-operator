package pki

import (
	"crypto/x509"
	"reflect"
	"testing"
	"time"
)

func TestCreateCA(t *testing.T) {
	testCreateCert(
		t,
		[]testCaseCreateCert{
			{
				name:     "No CommonName",
				x509Opts: []X509Opt{},
				wantErr:  true,
			},
			{
				name: "Invalid Lifetime",
				x509Opts: []X509Opt{
					WithNotBefore(time.Now().Add(2 * time.Hour)),
					WithNotAfter(time.Now().Add(1 * time.Hour)),
				},
				wantErr: true,
			},
			{
				name: "Valid",
				x509Opts: []X509Opt{
					WithCommonName("test"),
				},
				wantErr:        false,
				wantCommonName: "test",
				wantIssuer:     "test",
				wantDNSNames:   []string{"test"},
				wantKeyUsage:   x509.KeyUsageCertSign,
				wantIsCA:       true,
			},
			{
				name: "Custom Lifetime",
				x509Opts: []X509Opt{
					WithCommonName("test"),
					WithNotBefore(time.Now().Add(-2 * time.Hour)),
					WithNotAfter(time.Now().Add(5 * 365 * 24 * time.Hour)),
				},
				wantErr:        false,
				wantCommonName: "test",
				wantIssuer:     "test",
				wantDNSNames:   []string{"test"},
				wantKeyUsage:   x509.KeyUsageCertSign,
				wantIsCA:       true,
			},
		},
		CreateCA,
		ValidateCA,
	)
}

func TestCreateCert(t *testing.T) {
	caName := "tetst"
	caKeyPair, err := CreateCA(
		WithCommonName(caName),
	)
	if err != nil {
		t.Fatalf("CA cert creation should succeed. Got error: %v", err)
	}
	caCerts, err := caKeyPair.Certificates()
	if err != nil {
		t.Fatalf("Unable to get CA certificates: %v", err)
	}

	testCreateCert(
		t,
		[]testCaseCreateCert{
			{
				name: "Missing CommonName",
				x509Opts: []X509Opt{
					WithDNSNames("missing-common-name"),
				},
				wantErr: true,
			},
			{
				name: "Missing DNSNames",
				x509Opts: []X509Opt{
					WithCommonName("missing-dns-names"),
				},
				wantErr: true,
			},
			{
				name: "Invalid Lifetime",
				x509Opts: []X509Opt{
					WithCommonName("invalid-lifetime"),
					WithDNSNames("invalid-lifetime"),
					WithNotBefore(time.Now().Add(2 * time.Hour)),
					WithNotAfter(time.Now().Add(1 * time.Hour)),
				},
				wantErr: true,
			},
			{
				name: "Default Cert",
				x509Opts: []X509Opt{
					WithCommonName("default-cert"),
					WithDNSNames("default-cert"),
				},
				wantErr:        false,
				wantCommonName: "default-cert",
				wantIssuer:     caName,
				wantDNSNames:   []string{"default-cert"},
				wantKeyUsage:   x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
				wantIsCA:       false,
			},
			{
				name: "Custom Key Usage",
				x509Opts: []X509Opt{
					WithCommonName("custom-key-usage"),
					WithDNSNames("custom-key-usage"),
					WithKeyUsage(x509.KeyUsageKeyEncipherment),
				},
				wantErr:        false,
				wantCommonName: "custom-key-usage",
				wantIssuer:     caName,
				wantDNSNames:   []string{"custom-key-usage"},
				wantKeyUsage:   x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement | x509.KeyUsageKeyEncipherment,
				wantIsCA:       false,
			},
			{
				name: "Custom Ext Key Usage",
				x509Opts: []X509Opt{
					WithCommonName("custom-ext-key-usage"),
					WithDNSNames("custom-ext-key-usage"),
					WithExtKeyUsage(x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth),
				},
				wantErr:         false,
				wantCommonName:  "custom-ext-key-usage",
				wantIssuer:      caName,
				wantDNSNames:    []string{"custom-ext-key-usage"},
				wantKeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
				wantExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
				wantIsCA:        false,
			},
		},
		func(opts ...X509Opt) (*KeyPair, error) {
			return CreateCert(caKeyPair, opts...)
		},
		func(kp *KeyPair, dnsName string, at time.Time) (bool, error) {
			return ValidateCert(caCerts, kp, dnsName, at)
		},
	)
}

func TestValidateCert(t *testing.T) {
	rootCA := "test-root"
	rootKeyPair, err := CreateCA(
		WithCommonName(rootCA),
	)
	if err != nil {
		t.Fatalf("CA cert creation should succeed. Got error: %v", err)
	}

	intermediateCA := "test-intermediate"
	intermediateCAKeyPair, err := CreateCert(
		rootKeyPair,
		WithCommonName(intermediateCA),
		WithDNSNames(intermediateCA),
		WithKeyUsage(x509.KeyUsageCertSign),
		WithIsCA(true),
	)
	if err != nil {
		t.Fatalf("Intermediate CA cert creation should succeed. Got error: %v", err)
	}

	rootCerts, err := rootKeyPair.Certificates()
	if err != nil {
		t.Fatalf("Unable to get CA certificates: %v", err)
	}
	if len(rootCerts) != 1 {
		t.Fatalf("Unexpected number of root certs: %d", len(rootCerts))
	}
	rootCert := rootCerts[0]
	if rootCert.Subject.CommonName != rootCA {
		t.Fatalf("Unexpected root cert common name, got: %v, want: %v", rootCert.Subject.CommonName, rootCA)
	}
	if rootCert.Issuer.CommonName != rootCA {
		t.Fatalf("Unexpected root cert issuer, got: %v, want: %v", rootCert.Issuer.CommonName, rootCA)
	}

	intermediateCerts, err := intermediateCAKeyPair.Certificates()
	if err != nil {
		t.Fatalf("Unable to get intermediate CA certificates: %v", err)
	}
	if len(intermediateCerts) != 1 {
		t.Fatalf("Unexpected number of intermediate certs: %d", len(intermediateCerts))
	}
	intermediateCert := intermediateCerts[0]
	if intermediateCert.Subject.CommonName != intermediateCA {
		t.Fatalf("Unexpected intermediate cert common name, got: %v, want: %v", intermediateCert.Subject.CommonName, intermediateCA)
	}
	if intermediateCert.Issuer.CommonName != rootCA {
		t.Fatalf("Unexpected intermediate cert issuer, got: %v, want: %v", intermediateCert.Issuer.CommonName, rootCA)
	}

	tests := []struct {
		name                string
		createCertKeyPairFn func() *KeyPair
		dnsName             string
		at                  time.Time
		validateCertFn      func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error)
		wantValid           bool
		wantErr             bool
	}{
		{
			name: "CA invalid lifetime",
			createCertKeyPairFn: func() *KeyPair {
				return rootKeyPair
			},
			dnsName:        rootCA,
			at:             time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years in the future
			validateCertFn: ValidateCA,
			wantValid:      false,
			wantErr:        true,
		},
		{
			name: "CA invalid DNS name",
			createCertKeyPairFn: func() *KeyPair {
				return rootKeyPair
			},
			dnsName:        "foo",
			at:             time.Now(),
			validateCertFn: ValidateCA,
			wantValid:      false,
			wantErr:        true,
		},
		{
			name: "CA valid root",
			createCertKeyPairFn: func() *KeyPair {
				return rootKeyPair
			},
			dnsName:        rootCA,
			at:             time.Now(),
			validateCertFn: ValidateCA,
			wantValid:      true,
			wantErr:        false,
		},
		{
			name: "CA valid intermediate",
			createCertKeyPairFn: func() *KeyPair {
				return intermediateCAKeyPair
			},
			dnsName:        intermediateCA,
			at:             time.Now(),
			validateCertFn: ValidateCA,
			wantValid:      true,
			wantErr:        false,
		},
		{
			name: "Cert issued by root valid",
			createCertKeyPairFn: func() *KeyPair {
				return mustCreateCert(
					t,
					rootKeyPair,
					WithCommonName("issued-by-root"),
					WithDNSNames("issued-by-root"),
				)
			},
			dnsName: "issued-by-root",
			at:      time.Now(),
			validateCertFn: func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
				return ValidateCert(
					[]*x509.Certificate{
						rootCert,
					},
					keyPair,
					dnsName,
					at,
				)
			},
			wantValid: true,
			wantErr:   false,
		},
		{
			name: "Cert issued by root invalid trust chain",
			createCertKeyPairFn: func() *KeyPair {
				return mustCreateCert(
					t,
					rootKeyPair,
					WithCommonName("issued-by-root"),
					WithDNSNames("issued-by-root"),
				)
			},
			dnsName: "issued-by-root",
			at:      time.Now(),
			validateCertFn: func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
				return ValidateCert(
					[]*x509.Certificate{
						intermediateCert,
					},
					keyPair,
					dnsName,
					at,
				)
			},
			wantValid: false,
			wantErr:   true,
		},
		{
			name: "Cert issued by root invalid lifetime",
			createCertKeyPairFn: func() *KeyPair {
				return mustCreateCert(
					t,
					rootKeyPair,
					WithCommonName("issued-by-root"),
					WithDNSNames("issued-by-root"),
				)
			},
			dnsName: "issued-by-root",
			at:      time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years in the future
			validateCertFn: func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
				return ValidateCert(
					[]*x509.Certificate{
						rootCert,
					},
					keyPair,
					dnsName,
					at,
				)
			},
			wantValid: false,
			wantErr:   true,
		},
		{
			name: "Cert issued by root invalid DNS name",
			createCertKeyPairFn: func() *KeyPair {
				return mustCreateCert(
					t,
					rootKeyPair,
					WithCommonName("issued-by-root"),
					WithDNSNames("issued-by-root"),
				)
			},
			dnsName: "foo",
			at:      time.Now(),
			validateCertFn: func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
				return ValidateCert(
					[]*x509.Certificate{
						rootCert,
					},
					keyPair,
					dnsName,
					at,
				)
			},
			wantValid: false,
			wantErr:   true,
		},
		{
			name: "Cert issued by trusted intermediate valid",
			createCertKeyPairFn: func() *KeyPair {
				return mustCreateCert(
					t,
					intermediateCAKeyPair,
					WithCommonName("issued-by-intermediate"),
					WithDNSNames("issued-by-intermediate"),
				)
			},
			dnsName: "issued-by-intermediate",
			at:      time.Now(),
			validateCertFn: func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
				return ValidateCert(
					[]*x509.Certificate{
						intermediateCert,
					},
					keyPair,
					dnsName,
					at,
				)
			},
			wantValid: true,
			wantErr:   false,
		},
		{
			name: "Cert issued by untrusted intermediate valid",
			createCertKeyPairFn: func() *KeyPair {
				return mustCreateCert(
					t,
					intermediateCAKeyPair,
					WithCommonName("issued-by-intermediate"),
					WithDNSNames("issued-by-intermediate"),
				)
			},
			dnsName: "issued-by-intermediate",
			at:      time.Now(),
			validateCertFn: func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
				return ValidateCert(
					[]*x509.Certificate{
						rootCert,
					},
					keyPair,
					dnsName,
					at,
					WithIntermediateCAs(intermediateCert),
				)
			},
			wantValid: true,
			wantErr:   false,
		},
		{
			name: "Cert issued by untrusted intermediate invalid trust chain",
			createCertKeyPairFn: func() *KeyPair {
				return mustCreateCert(
					t,
					intermediateCAKeyPair,
					WithCommonName("issued-by-intermediate"),
					WithDNSNames("issued-by-intermediate"),
				)
			},
			dnsName: "issued-by-intermediate",
			at:      time.Now(),
			validateCertFn: func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
				return ValidateCert(
					[]*x509.Certificate{
						rootCert,
					},
					keyPair,
					dnsName,
					at,
				)
			},
			wantValid: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := tt.validateCertFn(tt.createCertKeyPairFn(), tt.dnsName, tt.at)
			if tt.wantErr && err == nil {
				t.Fatalf("Expecting error to be non nil for test '%s'", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Expecting error to be nil for test '%s'. Got: %v", tt.name, err)
			}
			if valid != tt.wantValid {
				t.Fatalf("Unexpected validation result for test '%s'. Got: %v, want: %v", tt.name, valid, tt.wantValid)
			}
		})
	}
}

func TestParseCertificates(t *testing.T) {
	tests := []struct {
		name      string
		certBytes []byte
		wantErr   bool
	}{
		{
			name:      "Invalid cert",
			certBytes: []byte("foo"),
			wantErr:   true,
		},
		{
			name: "No block cert",
			certBytes: []byte(`
MIID3DCCAsSgAwIBAgIBATANBgkqhkiG9w0BAQsFADA2MRkwFwYDVQQKExBtYXJp
YWRiLW9wZXJhdG9yMRkwFwYDVQQDExBtYXJpYWRiLW9wZXJhdG9yMB4XDTIzMTEw
NTEwMzAxNVoXDTIzMTEwNTEyMzAxNVowLzEtMCsGA1UEAxMkbWFyaWFkYi1vcGVy
YXRvci13ZWJob29rLmRlZmF1bHQuc3ZjMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEA1l202NMTln0/ngg4JXUJLJXvhSjjHimO22c47tHhWvnzhtnKCrH8
cWBnnxO11os5PcNIUYTxn04mZPRs+p1YkE9DMlp9Lgy/38304rr4kjllVspvl9Md
relqbcDy520rgF/YObfMZvzeseH2F5UK386IXb1KYSmp8dn7RU2HvUf17Z/z1Scd
vOS4xXPNjuAi28REA72vPbFwLbt+mQxBQ/Aal6BNH5RhNIOZ9m8fVsWn/e/4hZTa
2Ib/pp/3j2D1UlJqBiAh4cBeI0QYbj/hN5+OpVUJA3+OsGzFOBhs7KfqMAP3KDTt
7sTrPV03QKKqhDjh3LIzdZyEWHPMJesMawIDAQABo4H7MIH4MA4GA1UdDwEB/wQE
AwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMB8GA1UdIwQY
MBaAFCJFv64s92+rdv6JGeVbQLHBxXyUMIGhBgNVHREEgZkwgZaCMm1hcmlhZGIt
b3BlcmF0b3Itd2ViaG9vay5kZWZhdWx0LnN2Yy5jbHVzdGVyLmxvY2FsgiRtYXJp
YWRiLW9wZXJhdG9yLXdlYmhvb2suZGVmYXVsdC5zdmOCIG1hcmlhZGItb3BlcmF0
b3Itd2ViaG9vay5kZWZhdWx0ghhtYXJpYWRiLW9wZXJhdG9yLXdlYmhvb2swDQYJ
KoZIhvcNAQELBQADggEBABVoQWFqoB/wcdep9LlmWLqyVLy4Xx5mb0EhikvUKtE3
5ChDjiiQQEYdrXsBzxLsgntIh9XFx94eX2QtjOvDUCJc3z0LLg+5c5GhWANzvB7A
G1ZUYSKs5sgS0o5oBaPt9opZqnA8WGgwZ1WR1pxRBpLmu/019vDABAUX5tV3iqVp
qYxy6XmWp5Gc7c2NqlQ9N5xsMXMSfLiUSC8O+2sJGU92GtVSp7Vt4nGg1Qh5ZyHJ
fK6S3LzTZ/HVm8nXY1e0ZnrG7SZbcJkkZgSPOjsZ9KSikdG4I9+S99FTe8X1Qzn8
0ER77C84IUS9PEuvnSlWXopwKg5aAdHS5nHp7UFiNt4=
`),
			wantErr: true,
		},
		{
			name: "Valid cert",
			certBytes: []byte(`-----BEGIN CERTIFICATE-----
MIICXzCCAgagAwIBAgIRAIBgotjwHCDFrV2H9FWQrYIwCgYIKoZIzj0EAwIwNjEZ
MBcGA1UEChMQbWFyaWFkYi1vcGVyYXRvcjEZMBcGA1UEAxMQbWFyaWFkYi1vcGVy
YXRvcjAeFw0yNDEyMTkxNjI2NTVaFw0yNTEyMTkyMzI2NTVaMC8xLTArBgNVBAMT
JG1hcmlhZGItb3BlcmF0b3Itd2ViaG9vay5kZWZhdWx0LnN2YzBZMBMGByqGSM49
AgEGCCqGSM49AwEHA0IABIk1YZK4gZLLlluVtzL/S/dfJtQRAmh1Je2Vfz89qvOM
GPWhG8Xtjyd3Ntg7RBc4PXpjbq7lUufxy/oWp88+KPqjgfswgfgwDgYDVR0PAQH/
BAQDAgWgMBMGA1UdJQQMMAoGCCsGAQUFBwMBMAwGA1UdEwEB/wQCMAAwHwYDVR0j
BBgwFoAUXZzsgcecviPCXlVdrBw/tUEC2uYwgaEGA1UdEQSBmTCBloIybWFyaWFk
Yi1vcGVyYXRvci13ZWJob29rLmRlZmF1bHQuc3ZjLmNsdXN0ZXIubG9jYWyCJG1h
cmlhZGItb3BlcmF0b3Itd2ViaG9vay5kZWZhdWx0LnN2Y4IgbWFyaWFkYi1vcGVy
YXRvci13ZWJob29rLmRlZmF1bHSCGG1hcmlhZGItb3BlcmF0b3Itd2ViaG9vazAK
BggqhkjOPQQDAgNHADBEAiBSWY1rVufSE+3i0w553uJGJCC4Fpa6cvRPEti8X3Kp
1AIgG0qN5IT9EsRZaY4J2vBYsbN5LL+qRI5N0XGYqVWXuD8=
-----END CERTIFICATE-----			
`),
			wantErr: false,
		},
		{
			name: "Valid cert bundle",
			certBytes: []byte(`-----BEGIN CERTIFICATE-----
MIICFDCCAX2gAwIBAgIUAKik9DYK3ZWXZYqwYE310UeuqWowDQYJKoZIhvcNAQEL
BQAwHDEaMBgGA1UEAwwRbWFyaWFkYi1jbGllbnQtY2EwHhcNMjQxMTA4MTcxMTM0
WhcNMjUxMTA4MTcxMTM0WjAcMRowGAYDVQQDDBFtYXJpYWRiLWNsaWVudC1jYTCB
nzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAvMhNoeq4M/PXLbvkeeuegP3zWouG
u7a35kvXS0YPMhlQV08GcyDyKkt6cG4GrZ3bJUhtcqmzT8oqYKxb9T6W9HU5+gpr
BCScUWViCYX0pKhucEPHP/5xAJuGnnzg0BqR2Tzt95IDmg+tkFKGOnVn9Qx9RfXO
ZpEHL42pNSEU/9kCAwEAAaNTMFEwHQYDVR0OBBYEFCUdplOwmy91F9mlBbQ58UuN
ob4fMB8GA1UdIwQYMBaAFCUdplOwmy91F9mlBbQ58UuNob4fMA8GA1UdEwEB/wQF
MAMBAf8wDQYJKoZIhvcNAQELBQADgYEASsuxA5A5aVjl1QN/SrLGLIMOvcDnYdtW
HpZmElox1PR72AFV2H/Ig/9ixK+3DykMbDf6RiwMZBtgQVuHTRD8QoEk/gG5OEOP
VDiVGD+f28/5eme54pwI9FUuKxujP0pj4VPiCKR2igJcJnCIAeDTlNmcs7CiXtIn
WVQiuKIOhYk=
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIICFDCCAX2gAwIBAgIUCsM6MEeesw4qTYrp5laVrZhwopEwDQYJKoZIhvcNAQEL
BQAwHDEaMBgGA1UEAwwRbWFyaWFkYi1zZXJ2ZXItY2EwHhcNMjQxMTA4MTcxMTM0
WhcNMjUxMTA4MTcxMTM0WjAcMRowGAYDVQQDDBFtYXJpYWRiLXNlcnZlci1jYTCB
nzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAv8H2G9AKtM+tc0rR4GAm6CHYTffF
wLICdiUpcnLkqvMIU/YFsjBDFCbzUkmz7Fni176s1LH3tekBneRkFZ7hoyEwccbX
e3gBnnfGma7DzWvmRWMYf0dpnk4stOxZ44V/DJ2pSE7zI7zrH6w9dLRmJFcaQrQO
WWXkPnsQL3LArEECAwEAAaNTMFEwHQYDVR0OBBYEFN8WJNuBah6vZkrTjBESN+fc
yvLOMB8GA1UdIwQYMBaAFN8WJNuBah6vZkrTjBESN+fcyvLOMA8GA1UdEwEB/wQF
MAMBAf8wDQYJKoZIhvcNAQELBQADgYEAqymYNbFm/DX20eAkTBYyih6oAz5ETNJU
jDqaasPK77oFD2eEjSCI3jewj8xYaGfTgohB+YdkM9VWN+s5zsxBakTY19U7GeQJ
xj8tutwZ3pBj0lLiTnzYb6VnXpl12TiHImapwwAkZEpMZ3W3o0TjK2gyc6F9o2h/
idE60fGmuV8=
-----END CERTIFICATE-----
`),
			wantErr: false,
		},
		{
			name: "Invalid cert bundle",
			certBytes: []byte(`-----BEGIN CERTIFICATE-----
MIICFDCCAX2gAwIBAgIUAKik9DYK3ZWXZYqwYE310UeuqWowDQYJKoZIhvcNAQEL
BQAwHDEaMBgGA1UEAwwRbWFyaWFkYi1jbGllbnQtY2EwHhcNMjQxMTA4MTcxMTM0
WhcNMjUxMTA4MTcxMTM0WjAcMRowGAYDVQQDDBFtYXJpYWRiLWNsaWVudC1jYTCB
nzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAvMhNoeq4M/PXLbvkeeuegP3zWouG
u7a35kvXS0YPMhlQV08GcyDyKkt6cG4GrZ3bJUhtcqmzT8oqYKxb9T6W9HU5+gpr
BCScUWViCYX0pKhucEPHP/5xAJuGnnzg0BqR2Tzt95IDmg+tkFKGOnVn9Qx9RfXO
ZpEHL42pNSEU/9kCAwEAAaNTMFEwHQYDVR0OBBYEFCUdplOwmy91F9mlBbQ58UuN
ob4fMB8GA1UdIwQYMBaAFCUdplOwmy91F9mlBbQ58UuNob4fMA8GA1UdEwEB/wQF
MAMBAf8wDQYJKoZIhvcNAQELBQADgYEASsuxA5A5aVjl1QN/SrLGLIMOvcDnYdtW
HpZmElox1PR72AFV2H/Ig/9ixK+3DykMbDf6RiwMZBtgQVuHTRD8QoEk/gG5OEOP
VDiVGD+f28/5eme54pwI9FUuKxujP0pj4VPiCKR2igJcJnCIAeDTlNmcs7CiXtIn
WVQiuKIOhYk=
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIICFDCCAX2gAwIBAgIUCsM6MEeesw4qTYrp5laVrZhwopEwDQYJKoZIhvcNAQEL
BQAwHDEaMBgGA1UEAwwRbWFyaWFkYi1zZXJ2ZXItY2EwHhcNMjQxMTA4MTcxMTM0
WhcNMjUxMTA4MTcxMTM0WjAcMRowGAYDVQQDDBFtYXJpYWRiLXNlcnZlci1jYTCB
nzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAv8H2G9AKtM+tc0rR4GAm6CHYTffF
wLICdiUpcnLkqvMIU/YFsjBDFCbzUkmz7Fni176s1LH3tekBneRkFZ7hoyEwccbX
e3gBnnfGma7DzWvmRWMYf0dpnk4stOxZ44V/DJ2pSE7zI7zrH6w9dLRmJFcaQrQO
WWXkPnsQL3LArEECAwEAAaNTMFEwHQYDVR0OBBYEFN8WJNuBah6vZkrTjBESN+fc
yvLOMB8GA1UdIwQYMBaAFN8WJNuBah6vZkrTjBESN+fcyvLOMA8GA1UdEwEB/wQF
MAMBAf8wDQYJKoZIhvcNAQELBQADgYEAqymYNbFm/DX20eAkTBYyih6oAz5ETNJU
jDqaasPK77oFD2eEjSCI3jewj8xYaGfTgohB+YdkM9VWN+s5zsxBakTY19U7GeQJ
xj8tutwZ3pBj0lLiTnzYb6VnXpl12TiHImapwwAkZEpMZ3W3o0TjK2gyc6F9o2h/
idE60fGmuV8=
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
invalid
-----END CERTIFICATE-----
`),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCertificates(tt.certBytes)
			if tt.wantErr && err == nil {
				t.Fatalf("Expecting error to be non nil when parsing '%s'", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Expecting error to be nil when parsing '%s'. Got: %v", tt.name, err)
			}
		})
	}
}

func TestValidateLifetime(t *testing.T) {
	minLifetime := 1 * time.Hour
	maxLifetime := 5 * 365 * 24 * time.Hour // 5 years

	tests := []struct {
		name        string
		notBefore   time.Time
		notAfter    time.Time
		minDuration time.Duration
		maxDuration time.Duration
		wantErr     bool
	}{
		{
			name:        "Valid lifetime",
			notBefore:   time.Now(),
			notAfter:    time.Now().Add(2 * time.Hour),
			minDuration: minLifetime,
			maxDuration: maxLifetime,
			wantErr:     false,
		},
		{
			name:        "NotBefore after NotAfter",
			notBefore:   time.Now().Add(2 * time.Hour),
			notAfter:    time.Now(),
			minDuration: minLifetime,
			maxDuration: maxLifetime,
			wantErr:     true,
		},
		{
			name:        "Duration less than minimum",
			notBefore:   time.Now(),
			notAfter:    time.Now().Add(30 * time.Minute),
			minDuration: minLifetime,
			maxDuration: maxLifetime,
			wantErr:     true,
		},
		{
			name:        "Duration exceeds maximum",
			notBefore:   time.Now(),
			notAfter:    time.Now().Add(6 * 365 * 24 * time.Hour), // 6 years
			minDuration: minLifetime,
			maxDuration: maxLifetime,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLifetime(tt.notBefore, tt.notAfter, tt.minDuration, tt.maxDuration)
			if tt.wantErr && err == nil {
				t.Fatalf("Expecting error to be non nil for test '%s'", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Expecting error to be nil for test '%s'. Got: %v", tt.name, err)
			}
		})
	}
}

type testCaseCreateCert struct {
	name            string
	x509Opts        []X509Opt
	wantErr         bool
	wantCommonName  string
	wantIssuer      string
	wantDNSNames    []string
	wantKeyUsage    x509.KeyUsage
	wantExtKeyUsage []x509.ExtKeyUsage
	wantIsCA        bool
}

func testCreateCert(
	t *testing.T,
	tests []testCaseCreateCert,
	createCertFn func(...X509Opt) (*KeyPair, error),
	validateCertFn func(*KeyPair, string, time.Time) (bool, error),
) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyPair, err := createCertFn(tt.x509Opts...)
			if tt.wantErr && err == nil {
				t.Fatalf("Expecting error to be non nil when creating cert '%s'", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Expecting error to be nil when creating cert '%s'. Got: %v", tt.name, err)
			}
			if tt.wantErr {
				return
			}

			certs, err := keyPair.Certificates()
			if err != nil {
				t.Fatalf("error getting certificates: %v", err)
			}
			cert := certs[0] // we are only creating certificates with a single PEM block
			commonName := cert.Subject.CommonName
			notBefore := cert.NotBefore

			if commonName != tt.wantCommonName {
				t.Fatalf("unexpected common name, got: %v, want: %v", commonName, tt.wantCommonName)
			}
			if cert.Issuer.CommonName != tt.wantIssuer {
				t.Fatalf("unexpected issuer, got: %v, want: %v", cert.Issuer.CommonName, tt.wantIssuer)
			}
			if !reflect.DeepEqual(cert.DNSNames, tt.wantDNSNames) {
				t.Fatalf("unexpected DNS names, got: %v, want: %v", cert.DNSNames, tt.wantDNSNames)
			}
			if !reflect.DeepEqual(cert.KeyUsage, tt.wantKeyUsage) {
				t.Fatalf("unexpected key usage, got: %v, want: %v", cert.KeyUsage, tt.wantKeyUsage)
			}
			if !reflect.DeepEqual(cert.ExtKeyUsage, tt.wantExtKeyUsage) {
				t.Fatalf("unexpected extended key usage, got: %v, want: %v", cert.ExtKeyUsage, tt.wantExtKeyUsage)
			}
			if cert.IsCA != tt.wantIsCA {
				t.Fatalf("unexpected IsCA, got: %v, want: %v", cert.IsCA, tt.wantIsCA)
			}

			valid, err := validateCertFn(keyPair, commonName, notBefore.Add(-1*time.Hour))
			if err == nil {
				t.Fatalf("Cert validation should return an error. Got nil")
			}
			if valid {
				t.Fatal("Expected cert to be invalid")
			}

			valid, err = validateCertFn(keyPair, "foo", time.Now())
			if err == nil {
				t.Fatalf("Cert validation should return an error. Got nil")
			}
			if valid {
				t.Fatal("Expected cert to be invalid")
			}

			keyPair, err = createCertFn(tt.x509Opts...)
			if err != nil {
				t.Fatalf("Certificate renewal should succeed. Got error: %v", err)
			}

			valid, err = validateCertFn(keyPair, commonName, time.Now())
			if err != nil {
				t.Fatalf("Cert validation should succeed after renewal. Got error: %v", err)
			}
			if !valid {
				t.Fatal("Expected cert to be valid")
			}
		})
	}
}

func mustCreateCert(t *testing.T, caKeyPair *KeyPair, opts ...X509Opt) *KeyPair {
	keyPair, err := CreateCert(caKeyPair, opts...)
	if err != nil {
		t.Fatalf("unexpected error creating cert: %v", err)
	}
	return keyPair
}
