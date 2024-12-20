package pki

import (
	"crypto/x509"
	"testing"
	"time"
)

func TestCACert(t *testing.T) {
	caName := "test-mariadb-operator"
	x509Opts := []X509Opt{
		WithCommonName(caName),
		WithNotBefore(time.Now()),
		WithNotAfter(time.Now().Add(24 * time.Hour)),
	}
	caKeyPair, err := CreateCA(x509Opts...)
	if err != nil {
		t.Fatalf("CA cert creation should succeed. Got error: %v", err)
	}

	valid, err := ValidCACert(caKeyPair, caName, time.Now())
	if err != nil {
		t.Fatalf("CA cert validation should succeed. Got error: %v", err)
	}
	if !valid {
		t.Fatal("Expected CA cert to be valid")
	}

	valid, err = ValidCACert(caKeyPair, caName, time.Now().Add(-1*time.Hour))
	if err == nil {
		t.Fatalf("CA cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected CA cert to be invalid")
	}

	valid, err = ValidCACert(caKeyPair, "foo", time.Now())
	if err == nil {
		t.Fatalf("CA cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected CA cert to be invalid")
	}

	caKeyPair, err = CreateCA(x509Opts...)
	if err != nil {
		t.Fatalf("CA cert renewal should succeed. Got error: %v", err)
	}

	valid, err = ValidCACert(caKeyPair, caName, time.Now())
	if err != nil {
		t.Fatalf("CA cert validation should succeed after renewal. Got error: %v", err)
	}
	if !valid {
		t.Fatal("Expected CA cert to be valid after renewal")
	}
}

func TestCert(t *testing.T) {
	caKeyPair, err := CreateCA()
	if err != nil {
		t.Fatalf("CA cert creation should succeed. Got error: %v", err)
	}
	caCerts, err := caKeyPair.Certificates()
	if err != nil {
		t.Fatalf("Unable to get CA certificates: %v", err)
	}

	commonName := "mariadb-operator.default.svc"
	x509Opts := []X509Opt{
		WithCommonName(commonName),
		WithDNSNames([]string{
			"mariadb-operator",
			"mariadb-operator.default",
			commonName,
		}),
		WithNotBefore(time.Now()),
		WithNotAfter(time.Now().Add(24 * time.Hour)),
		WithExtKeyUsage(x509.ExtKeyUsageServerAuth),
	}
	keyPairPEM, err := CreateCert(caKeyPair, x509Opts...)
	if err != nil {
		t.Fatalf("Certificate creation should succeed. Got error: %v", err)
	}

	valid, err := ValidCert(caCerts, keyPairPEM, commonName, time.Now())
	if err != nil {
		t.Fatalf("Cert validation should succeed. Got error: %v", err)
	}
	if !valid {
		t.Fatal("Expected cert to be valid")
	}

	valid, err = ValidCert(caCerts, keyPairPEM, commonName, time.Now().Add(-1*time.Hour))
	if err == nil {
		t.Fatalf("Cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected cert to be invalid")
	}

	valid, err = ValidCert(caCerts, keyPairPEM, "foo", time.Now())
	if err == nil {
		t.Fatalf("Cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected cert to be invalid")
	}

	keyPairPEM, err = CreateCert(caKeyPair, x509Opts...)
	if err != nil {
		t.Fatalf("Certificate renewal should succeed. Got error: %v", err)
	}

	valid, err = ValidCert(caCerts, keyPairPEM, commonName, time.Now())
	if err != nil {
		t.Fatalf("Cert validation should succeed after renewal. Got error: %v", err)
	}
	if !valid {
		t.Fatal("Expected cert to be valid")
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
