package pki

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testTLSCert = `
-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----	
`
	testTLSCertBundle = `
-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----	
`
	testTLSCertNoBlock = `
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
`
	testTLSKey = `
-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEA1l202NMTln0/ngg4JXUJLJXvhSjjHimO22c47tHhWvnzhtnK
CrH8cWBnnxO11os5PcNIUYTxn04mZPRs+p1YkE9DMlp9Lgy/38304rr4kjllVspv
l9MdrelqbcDy520rgF/YObfMZvzeseH2F5UK386IXb1KYSmp8dn7RU2HvUf17Z/z
1ScdvOS4xXPNjuAi28REA72vPbFwLbt+mQxBQ/Aal6BNH5RhNIOZ9m8fVsWn/e/4
hZTa2Ib/pp/3j2D1UlJqBiAh4cBeI0QYbj/hN5+OpVUJA3+OsGzFOBhs7KfqMAP3
KDTt7sTrPV03QKKqhDjh3LIzdZyEWHPMJesMawIDAQABAoIBAEw5tf0D0YtJrj17
nrtzCngYOLuY9mnbTTknU09YwlGfX8Er4HQ9Jg8KwM4ILDjF+OzFbAnQxDpph62O
XNIg8UUfaj2Vf73IOtJSYindYlZconRiN5w9LeiRf47XdYhlgXp8ml6rxLs6X9XR
C7kG/n7m6garMK+sKQofAQJ7tzDOpzna/V+C3z7vwupmypjVmKuqFfDOITUhk06Q
WNUbiBSRbVtuWJ4+xpg5cZbCSIzlziQFbaRjQ0bHVIoJ/DrHI8OCEELjcQi7aMPX
Z6XZnpVteae5o5lpw1AbB7c4ALF5B4Z1RQKakb+mK7o4F6SV/TOf2Fb7Gwp5gSOO
Udo/acECgYEA7woaxJvkyJH6G83kt73RUudFbKIUo/lv8E0BLcN/7jo8zVMQIoxN
m6fffVWfy2zVd5rLus4wjq+f5jmAprJykkA7n3HIM6HJ2v9KJPVCBKA8bgJ5eU0Y
c7VFz8O0Z8dj/p9oVrHLo0Aqo+69rXG6wcyck1au3FEw16jflU1MaMUCgYEA5ZNv
fHHDtWDjCPM75N7aaw2C9ENL3fi8Fwh/h4ZRj/BftHnHZkjY3bpW1/17lXuTnbE6
uHeq3s/wBt4+F/N61ps1a7NTOtZHst5fPZZUjWuT0vq0EN2KB81iR/N4ld7w3F3v
bZSPNXpm1J3wSAVrL1tHdQdWXdkOTeTA4wAnE28CgYANZKuLSJDRDBzPYgHmqaQI
2RxyscImTduPw0DFp6aLWof9mSHWTbYreoRzKVECvN5ZDTtNBDCETiLPa3lh3a29
tAujK2TkP7RnqNYmq/c++xtnrovP2Bn+obF/qp95ERrxMU1PTjbytq2s8bt+9Fha
c3RybPDvNz1dWADvBJ27YQKBgAF6b49XlDEIzK10E4CnxrRFxAAaptRpE5z6Wwfe
X4wTuioJVrVb5rmWx5Rgd3lA8HRlfcFOU/VXVW5V5AR3duUG3tMwtmp8kr2eHPLi
kuzOMod7QcmSA5+FPQrFkJM2ekqQ+Ee2Wy22+g6IbdGo50XIyq8AOxgjm6n4vR05
FQdVAoGAUt0hi789SX+BKzkWWMnds8HZzO3a5OgN9on+Hd/BVaQG6+F4wBGyjdjz
PEZhWvsvx9Qge+DhwW3vfTDH2RItevD5Av4x0jZX0TWPoII8aP07VmDQ4g0cEkKh
nBZoGLkeDofSc+Ml4HRpi43U+fqhU77wr8Gq0YU74h7lFfiRI/M=
-----END RSA PRIVATE KEY-----
`
)

func TestKeyPairFromTLSSecret(t *testing.T) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte(testTLSCert),
			"tls.key": []byte(testTLSKey),
		},
	}

	keyPair, err := KeyPairFromTLSSecret(&secret)
	if err != nil {
		t.Fatalf("Unexpected error creating KeyPair from TLS Secret: %v", err)
	}
	if keyPair == nil {
		t.Fatal("KeyPair should nit be nul")
	}
	expectedCN := "mariadb-operator-webhook.default.svc"
	if expectedCN != keyPair.Cert.Subject.CommonName {
		t.Fatalf("Expected CommonName to be %v. Got %v", expectedCN, keyPair.Cert.Subject.CommonName)
	}
}

func TestKeyPairInvalidPEM(t *testing.T) {
	_, err := KeyPairFromPEM([]byte("foo"), []byte("bar"))
	if err == nil {
		t.Fatal("Expected KeyPair creation to fail")
	}
}

func TestCACert(t *testing.T) {
	caName := "test-mariadb-operator"
	x509Opts := []X509Opt{
		WithCommonName(caName),
		WithOrganization("test-org"),
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
	}
	keyPairPEM, err := CreateCert(caKeyPair, x509Opts...)
	if err != nil {
		t.Fatalf("Certificate creation should succeed. Got error: %v", err)
	}

	valid, err := ValidCert(caKeyPair.Cert, keyPairPEM, commonName, time.Now())
	if err != nil {
		t.Fatalf("Cert validation should succeed. Got error: %v", err)
	}
	if !valid {
		t.Fatal("Expected cert to be valid")
	}

	valid, err = ValidCert(caKeyPair.Cert, keyPairPEM, commonName, time.Now().Add(-1*time.Hour))
	if err == nil {
		t.Fatalf("Cert validation should return an error. Got nil")
	}
	if valid {
		t.Fatal("Expected cert to be invalid")
	}

	valid, err = ValidCert(caKeyPair.Cert, keyPairPEM, "foo", time.Now())
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

	valid, err = ValidCert(caKeyPair.Cert, keyPairPEM, commonName, time.Now())
	if err != nil {
		t.Fatalf("Cert validation should succeed after renewal. Got error: %v", err)
	}
	if !valid {
		t.Fatal("Expected cert to be valid")
	}
}

func TestParseCert(t *testing.T) {
	tests := []struct {
		name      string
		certBytes []byte
		wantErr   bool
	}{
		{
			name:      "Valid cert",
			certBytes: []byte(testTLSCert),
			wantErr:   false,
		},
		{
			name:      "Valid cert bundle",
			certBytes: []byte(testTLSCertBundle),
			wantErr:   false,
		},
		{
			name:      "No block cert",
			certBytes: []byte(testTLSCertNoBlock),
			wantErr:   true,
		},
		{
			name:      "Invalid cert",
			certBytes: []byte("foo"),
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCert(tt.certBytes)
			if tt.wantErr && err == nil {
				t.Fatalf("Expecting error to be non nil when parsing '%s'", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Expecting error to be nil when parsing '%s'. Got: %v", tt.name, err)
			}
		})
	}
}
