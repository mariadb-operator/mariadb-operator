package pki

import (
	"crypto/x509"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type testCaseCreateCert struct {
	x509Opts        []X509Opt
	wantErr         bool
	wantCommonName  string
	wantIssuer      string
	wantDNSNames    []string
	wantKeyUsage    x509.KeyUsage
	wantExtKeyUsage []x509.ExtKeyUsage
	wantIsCA        bool
}

func runCreateCertCase(
	tt testCaseCreateCert,
	createCertFn func(...X509Opt) (*KeyPair, error),
	validateCertFn func(*KeyPair, string, time.Time) (bool, error),
) {
	keyPair, err := createCertFn(tt.x509Opts...)
	if tt.wantErr {
		Expect(err).To(HaveOccurred())
	} else {
		Expect(err).NotTo(HaveOccurred())
	}
	if tt.wantErr {
		return
	}

	cert, err := keyPair.LeafCertificate() // we are only creating certificates with a single leaf cert, therefore only one PEM block
	Expect(err).NotTo(HaveOccurred())
	commonName := cert.Subject.CommonName
	notBefore := cert.NotBefore

	Expect(commonName).To(Equal(tt.wantCommonName))
	Expect(cert.Issuer.CommonName).To(Equal(tt.wantIssuer))
	Expect(cert.DNSNames).To(Equal(tt.wantDNSNames))
	Expect(cert.KeyUsage).To(Equal(tt.wantKeyUsage))
	Expect(cert.ExtKeyUsage).To(Equal(tt.wantExtKeyUsage))
	Expect(cert.IsCA).To(Equal(tt.wantIsCA))

	valid, err := validateCertFn(keyPair, commonName, notBefore.Add(-1*time.Hour))
	Expect(err).To(HaveOccurred())
	Expect(valid).To(BeFalse())

	valid, err = validateCertFn(keyPair, "foo", time.Now())
	Expect(err).To(HaveOccurred())
	Expect(valid).To(BeFalse())

	keyPair, err = createCertFn(tt.x509Opts...)
	Expect(err).NotTo(HaveOccurred())

	valid, err = validateCertFn(keyPair, commonName, time.Now())
	Expect(err).NotTo(HaveOccurred())
	Expect(valid).To(BeTrue())
}

func mustCreateCert(caKeyPair *KeyPair, opts ...X509Opt) *KeyPair {
	keyPair, err := CreateCert(caKeyPair, opts...)
	Expect(err).NotTo(HaveOccurred())
	return keyPair
}

var _ = Describe("CreateCA", func() {
	DescribeTable("creates a CA certificate",
		func(tt testCaseCreateCert) {
			runCreateCertCase(tt, CreateCA, ValidateCA)
		},
		Entry("No CommonName", testCaseCreateCert{
			x509Opts: []X509Opt{},
			wantErr:  true,
		}),
		Entry("Invalid Lifetime", testCaseCreateCert{
			x509Opts: []X509Opt{
				WithNotBefore(time.Now().Add(2 * time.Hour)),
				WithNotAfter(time.Now().Add(1 * time.Hour)),
			},
			wantErr: true,
		}),
		Entry("Valid", testCaseCreateCert{
			x509Opts: []X509Opt{
				WithCommonName("test"),
			},
			wantErr:        false,
			wantCommonName: "test",
			wantIssuer:     "test",
			wantDNSNames:   []string{"test"},
			wantKeyUsage:   x509.KeyUsageCertSign,
			wantIsCA:       true,
		}),
		Entry("Custom Lifetime", testCaseCreateCert{
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
		}),
	)
})

var _ = Describe("CreateCert", func() {
	caName := "tetst"
	var (
		caKeyPair *KeyPair
		caCerts   []*x509.Certificate
	)
	BeforeEach(func() {
		var err error
		caKeyPair, err = CreateCA(
			WithCommonName(caName),
		)
		Expect(err).NotTo(HaveOccurred())
		caCerts, err = caKeyPair.Certificates()
		Expect(err).NotTo(HaveOccurred())
	})

	DescribeTable("creates a certificate signed by a CA",
		func(tt testCaseCreateCert) {
			runCreateCertCase(
				tt,
				func(opts ...X509Opt) (*KeyPair, error) {
					return CreateCert(caKeyPair, opts...)
				},
				func(kp *KeyPair, dnsName string, at time.Time) (bool, error) {
					return ValidateCert(caCerts, kp, dnsName, at)
				},
			)
		},
		Entry("Missing CommonName", testCaseCreateCert{
			x509Opts: []X509Opt{
				WithDNSNames("missing-common-name"),
			},
			wantErr: true,
		}),
		Entry("Missing DNSNames", testCaseCreateCert{
			x509Opts: []X509Opt{
				WithCommonName("missing-dns-names"),
			},
			wantErr: true,
		}),
		Entry("Invalid Lifetime", testCaseCreateCert{
			x509Opts: []X509Opt{
				WithCommonName("invalid-lifetime"),
				WithDNSNames("invalid-lifetime"),
				WithNotBefore(time.Now().Add(2 * time.Hour)),
				WithNotAfter(time.Now().Add(1 * time.Hour)),
			},
			wantErr: true,
		}),
		Entry("Default Cert", testCaseCreateCert{
			x509Opts: []X509Opt{
				WithCommonName("default-cert"),
				WithDNSNames("default-cert"),
			},
			wantErr:        false,
			wantCommonName: "default-cert",
			wantIssuer:     "tetst",
			wantDNSNames:   []string{"default-cert"},
			wantKeyUsage:   x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
			wantIsCA:       false,
		}),
		Entry("Custom Key Usage", testCaseCreateCert{
			x509Opts: []X509Opt{
				WithCommonName("custom-key-usage"),
				WithDNSNames("custom-key-usage"),
				WithKeyUsage(x509.KeyUsageKeyEncipherment),
			},
			wantErr:        false,
			wantCommonName: "custom-key-usage",
			wantIssuer:     "tetst",
			wantDNSNames:   []string{"custom-key-usage"},
			wantKeyUsage:   x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement | x509.KeyUsageKeyEncipherment,
			wantIsCA:       false,
		}),
		Entry("Custom Ext Key Usage", testCaseCreateCert{
			x509Opts: []X509Opt{
				WithCommonName("custom-ext-key-usage"),
				WithDNSNames("custom-ext-key-usage"),
				WithExtKeyUsage(x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth),
			},
			wantErr:         false,
			wantCommonName:  "custom-ext-key-usage",
			wantIssuer:      "tetst",
			wantDNSNames:    []string{"custom-ext-key-usage"},
			wantKeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
			wantExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
			wantIsCA:        false,
		}),
	)
})

var _ = Describe("ValidateCert", func() {
	rootCA := "test-root"
	intermediateCA := "test-intermediate"
	var (
		rootKeyPair           *KeyPair
		intermediateCAKeyPair *KeyPair
		rootCert              *x509.Certificate
		intermediateCert      *x509.Certificate
	)
	BeforeEach(func() {
		var err error
		rootKeyPair, err = CreateCA(
			WithCommonName(rootCA),
		)
		Expect(err).NotTo(HaveOccurred())

		intermediateCAKeyPair, err = CreateCert(
			rootKeyPair,
			WithCommonName(intermediateCA),
			WithDNSNames(intermediateCA),
			WithKeyUsage(x509.KeyUsageCertSign),
			WithIsCA(true),
		)
		Expect(err).NotTo(HaveOccurred())

		rootCerts, err := rootKeyPair.Certificates()
		Expect(err).NotTo(HaveOccurred())
		Expect(rootCerts).To(HaveLen(1))
		rootCert = rootCerts[0]
		Expect(rootCert.Subject.CommonName).To(Equal(rootCA))
		Expect(rootCert.Issuer.CommonName).To(Equal(rootCA))

		intermediateCerts, err := intermediateCAKeyPair.Certificates()
		Expect(err).NotTo(HaveOccurred())
		Expect(intermediateCerts).To(HaveLen(1))
		intermediateCert = intermediateCerts[0]
		Expect(intermediateCert.Subject.CommonName).To(Equal(intermediateCA))
		Expect(intermediateCert.Issuer.CommonName).To(Equal(rootCA))
	})

	DescribeTable("validates a certificate",
		func(
			createCertKeyPairFn func() *KeyPair,
			dnsName string,
			at time.Time,
			validateCertFn func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error),
			wantValid bool,
			wantErr bool,
		) {
			valid, err := validateCertFn(createCertKeyPairFn(), dnsName, at)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(valid).To(Equal(wantValid))
		},
		Entry("CA invalid lifetime",
			func() *KeyPair {
				return rootKeyPair
			},
			rootCA,
			time.Now().Add(10*365*24*time.Hour), // 10 years in the future
			ValidateCA,
			false,
			true,
		),
		Entry("CA invalid DNS name",
			func() *KeyPair {
				return rootKeyPair
			},
			"foo",
			time.Now(),
			ValidateCA,
			false,
			true,
		),
		Entry("CA valid root",
			func() *KeyPair {
				return rootKeyPair
			},
			rootCA,
			time.Now(),
			ValidateCA,
			true,
			false,
		),
		Entry("CA valid intermediate",
			func() *KeyPair {
				return intermediateCAKeyPair
			},
			intermediateCA,
			time.Now(),
			ValidateCA,
			true,
			false,
		),
		Entry("Cert issued by root valid",
			func() *KeyPair {
				return mustCreateCert(
					rootKeyPair,
					WithCommonName("issued-by-root"),
					WithDNSNames("issued-by-root"),
				)
			},
			"issued-by-root",
			time.Now(),
			func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
				return ValidateCert(
					[]*x509.Certificate{
						rootCert,
					},
					keyPair,
					dnsName,
					at,
				)
			},
			true,
			false,
		),
		Entry("Cert issued by root invalid trust chain",
			func() *KeyPair {
				return mustCreateCert(
					rootKeyPair,
					WithCommonName("issued-by-root"),
					WithDNSNames("issued-by-root"),
				)
			},
			"issued-by-root",
			time.Now(),
			func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
				return ValidateCert(
					[]*x509.Certificate{
						intermediateCert,
					},
					keyPair,
					dnsName,
					at,
				)
			},
			false,
			true,
		),
		Entry("Cert issued by root invalid lifetime",
			func() *KeyPair {
				return mustCreateCert(
					rootKeyPair,
					WithCommonName("issued-by-root"),
					WithDNSNames("issued-by-root"),
				)
			},
			"issued-by-root",
			time.Now().Add(10*365*24*time.Hour), // 10 years in the future
			func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
				return ValidateCert(
					[]*x509.Certificate{
						rootCert,
					},
					keyPair,
					dnsName,
					at,
				)
			},
			false,
			true,
		),
		Entry("Cert issued by root invalid DNS name",
			func() *KeyPair {
				return mustCreateCert(
					rootKeyPair,
					WithCommonName("issued-by-root"),
					WithDNSNames("issued-by-root"),
				)
			},
			"foo",
			time.Now(),
			func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
				return ValidateCert(
					[]*x509.Certificate{
						rootCert,
					},
					keyPair,
					dnsName,
					at,
				)
			},
			false,
			true,
		),
		Entry("Cert issued by trusted intermediate valid",
			func() *KeyPair {
				return mustCreateCert(
					intermediateCAKeyPair,
					WithCommonName("issued-by-intermediate"),
					WithDNSNames("issued-by-intermediate"),
				)
			},
			"issued-by-intermediate",
			time.Now(),
			func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
				return ValidateCert(
					[]*x509.Certificate{
						intermediateCert,
					},
					keyPair,
					dnsName,
					at,
				)
			},
			true,
			false,
		),
		Entry("Cert issued by untrusted intermediate valid",
			func() *KeyPair {
				return mustCreateCert(
					intermediateCAKeyPair,
					WithCommonName("issued-by-intermediate"),
					WithDNSNames("issued-by-intermediate"),
				)
			},
			"issued-by-intermediate",
			time.Now(),
			func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
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
			true,
			false,
		),
		Entry("Cert issued by untrusted intermediate invalid trust chain",
			func() *KeyPair {
				return mustCreateCert(
					intermediateCAKeyPair,
					WithCommonName("issued-by-intermediate"),
					WithDNSNames("issued-by-intermediate"),
				)
			},
			"issued-by-intermediate",
			time.Now(),
			func(keyPair *KeyPair, dnsName string, at time.Time) (bool, error) {
				return ValidateCert(
					[]*x509.Certificate{
						rootCert,
					},
					keyPair,
					dnsName,
					at,
				)
			},
			false,
			true,
		),
	)
})

var _ = Describe("ParseCertificates", func() {
	DescribeTable("parses certificates",
		func(certBytes []byte, wantErr bool) {
			_, err := ParseCertificates(certBytes)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Invalid cert", []byte("foo"), true),
		Entry("No block cert", []byte(`
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
`), true),
		Entry("Valid cert", []byte(`-----BEGIN CERTIFICATE-----
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
`), false),
		Entry("Valid cert bundle", []byte(`-----BEGIN CERTIFICATE-----
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
`), false),
		Entry("Invalid cert bundle", []byte(`-----BEGIN CERTIFICATE-----
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
`), true),
	)
})

var _ = Describe("validateLifetime", func() {
	minLifetime := 1 * time.Hour
	maxLifetime := 5 * 365 * 24 * time.Hour // 5 years

	DescribeTable("validates a certificate lifetime",
		func(notBefore, notAfter time.Time, minDuration, maxDuration time.Duration, wantErr bool) {
			err := validateLifetime(notBefore, notAfter, minDuration, maxDuration)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Valid lifetime",
			time.Now(),
			time.Now().Add(2*time.Hour),
			minLifetime,
			maxLifetime,
			false,
		),
		Entry("NotBefore after NotAfter",
			time.Now().Add(2*time.Hour),
			time.Now(),
			minLifetime,
			maxLifetime,
			true,
		),
		Entry("Duration less than minimum",
			time.Now(),
			time.Now().Add(30*time.Minute),
			minLifetime,
			maxLifetime,
			true,
		),
		Entry("Duration exceeds maximum",
			time.Now(),
			time.Now().Add(6*365*24*time.Hour), // 6 years
			minLifetime,
			maxLifetime,
			true,
		),
	)
})
