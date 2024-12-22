package pki

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewKeyPair(t *testing.T) {
	tests := []struct {
		name    string
		certPEM []byte
		keyPEM  []byte
		opts    []KeyPairOpt
		wantErr bool
	}{
		{
			name:    "Empty Cert PEM",
			certPEM: []byte(""),
			keyPEM: []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIAdp3iKnNA1kO2Ep5Hw7owMcm06SecFGdOqW/vO4k2AjoAoGCCqGSM49
AwEHoUQDQgAEiTVhkriBksuWW5W3Mv9L918m1BECaHUl7ZV/Pz2q84wY9aEbxe2P
J3c22DtEFzg9emNuruVS5/HL+hanzz4o+g==
-----END EC PRIVATE KEY-----
`),
			opts:    []KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeECDSA)},
			wantErr: true,
		},
		{
			name: "Empty Key PEM",
			certPEM: []byte(`-----BEGIN CERTIFICATE-----
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
			keyPEM:  []byte(""),
			opts:    []KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeECDSA)},
			wantErr: true,
		},
		{
			name:    "Invalid Cert PEM",
			certPEM: []byte("invalid-cert"),
			keyPEM: []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIAdp3iKnNA1kO2Ep5Hw7owMcm06SecFGdOqW/vO4k2AjoAoGCCqGSM49
AwEHoUQDQgAEiTVhkriBksuWW5W3Mv9L918m1BECaHUl7ZV/Pz2q84wY9aEbxe2P
J3c22DtEFzg9emNuruVS5/HL+hanzz4o+g==
-----END EC PRIVATE KEY-----
`),
			opts:    []KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeECDSA)},
			wantErr: true,
		},
		{
			name: "Invalid Key PEM",
			certPEM: []byte(`-----BEGIN CERTIFICATE-----
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
			keyPEM:  []byte("invalid-key"),
			opts:    []KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeRSA)},
			wantErr: true,
		},
		{
			name: "Valid",
			certPEM: []byte(`-----BEGIN CERTIFICATE-----
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
			keyPEM: []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIAdp3iKnNA1kO2Ep5Hw7owMcm06SecFGdOqW/vO4k2AjoAoGCCqGSM49
AwEHoUQDQgAEiTVhkriBksuWW5W3Mv9L918m1BECaHUl7ZV/Pz2q84wY9aEbxe2P
J3c22DtEFzg9emNuruVS5/HL+hanzz4o+g==
-----END EC PRIVATE KEY-----
`),
			opts:    []KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeECDSA)},
			wantErr: false,
		},
		{
			name: "Unmatched",
			certPEM: []byte(`-----BEGIN CERTIFICATE-----
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
			keyPEM: []byte(`-----BEGIN RSA PRIVATE KEY-----
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
`),
			opts:    []KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeRSA)},
			wantErr: true,
		},
		{
			name: "Unsupported",
			certPEM: []byte(`-----BEGIN CERTIFICATE-----
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
			keyPEM: []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIAdp3iKnNA1kO2Ep5Hw7owMcm06SecFGdOqW/vO4k2AjoAoGCCqGSM49
AwEHoUQDQgAEiTVhkriBksuWW5W3Mv9L918m1BECaHUl7ZV/Pz2q84wY9aEbxe2P
J3c22DtEFzg9emNuruVS5/HL+hanzz4o+g==
-----END EC PRIVATE KEY-----
`),
			opts:    []KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeRSA)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewKeyPair(tt.certPEM, tt.keyPEM, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyPair() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewKeyPairFromSecret(t *testing.T) {
	tests := []struct {
		name          string
		secret        *corev1.Secret
		certKey       string
		privateKeyKey string
		opts          []KeyPairOpt
		wantErr       bool
	}{
		{
			name: "Empty Secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Data: map[string][]byte{},
			},
			certKey:       TLSCertKey,
			privateKeyKey: TLSKeyKey,
			wantErr:       true,
		},
		{
			name: "Missing Cert Key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Data: map[string][]byte{
					TLSKeyKey: []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIAdp3iKnNA1kO2Ep5Hw7owMcm06SecFGdOqW/vO4k2AjoAoGCCqGSM49
AwEHoUQDQgAEiTVhkriBksuWW5W3Mv9L918m1BECaHUl7ZV/Pz2q84wY9aEbxe2P
J3c22DtEFzg9emNuruVS5/HL+hanzz4o+g==
-----END EC PRIVATE KEY-----
`),
				},
			},
			certKey:       TLSCertKey,
			privateKeyKey: TLSKeyKey,
			wantErr:       true,
		},
		{
			name: "Missing Key Key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Data: map[string][]byte{
					TLSCertKey: []byte(`-----BEGIN CERTIFICATE-----
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
				},
			},
			certKey:       TLSCertKey,
			privateKeyKey: TLSKeyKey,
			wantErr:       true,
		},
		{
			name: "Invalid Cert PEM",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Data: map[string][]byte{
					TLSCertKey: []byte("invalid-cert"),
					TLSKeyKey: []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIAdp3iKnNA1kO2Ep5Hw7owMcm06SecFGdOqW/vO4k2AjoAoGCCqGSM49
AwEHoUQDQgAEiTVhkriBksuWW5W3Mv9L918m1BECaHUl7ZV/Pz2q84wY9aEbxe2P
J3c22DtEFzg9emNuruVS5/HL+hanzz4o+g==
-----END EC PRIVATE KEY-----
`),
				},
			},
			certKey:       TLSCertKey,
			privateKeyKey: TLSKeyKey,
			wantErr:       true,
		},
		{
			name: "Invalid Key PEM",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Data: map[string][]byte{
					TLSCertKey: []byte(`-----BEGIN CERTIFICATE-----
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
					TLSKeyKey: []byte("invalid-key"),
				},
			},
			certKey:       TLSCertKey,
			privateKeyKey: TLSKeyKey,
			wantErr:       true,
		},
		{
			name: "Valid",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Data: map[string][]byte{
					TLSCertKey: []byte(`-----BEGIN CERTIFICATE-----
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
					TLSKeyKey: []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIAdp3iKnNA1kO2Ep5Hw7owMcm06SecFGdOqW/vO4k2AjoAoGCCqGSM49
AwEHoUQDQgAEiTVhkriBksuWW5W3Mv9L918m1BECaHUl7ZV/Pz2q84wY9aEbxe2P
J3c22DtEFzg9emNuruVS5/HL+hanzz4o+g==
-----END EC PRIVATE KEY-----
`),
				},
			},
			certKey:       TLSCertKey,
			privateKeyKey: TLSKeyKey,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewKeyPairFromSecret(tt.secret, tt.certKey, tt.privateKeyKey, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyPairFromSecret() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewKeyPairFromTemplate(t *testing.T) {
	// Subject: CN=mariadb-operator
	caCertPEm := []byte(`-----BEGIN CERTIFICATE-----
MIIByjCCAW+gAwIBAgIQInuHavw36CvWL7osylp/9jAKBggqhkjOPQQDAjA2MRkw
FwYDVQQKExBtYXJpYWRiLW9wZXJhdG9yMRkwFwYDVQQDExBtYXJpYWRiLW9wZXJh
dG9yMB4XDTI0MTIxOTE2MjY1NVoXDTI4MTIxOTE3MjY1NVowNjEZMBcGA1UEChMQ
bWFyaWFkYi1vcGVyYXRvcjEZMBcGA1UEAxMQbWFyaWFkYi1vcGVyYXRvcjBZMBMG
ByqGSM49AgEGCCqGSM49AwEHA0IABIx2WSuRfc98PRJB8+7IkFjLBh0jqdQXWwLt
HW2tYw+MrFJthf93kcDH122iAUsjZ0/nvf4JR0LFJmFS7uTLJJijXzBdMA4GA1Ud
DwEB/wQEAwICpDAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRdnOyBx5y+I8Je
VV2sHD+1QQLa5jAbBgNVHREEFDASghBtYXJpYWRiLW9wZXJhdG9yMAoGCCqGSM49
BAMCA0kAMEYCIQClC63wr9jTwqhd8DKuKN5riMgoW4vXxvbsBfKuoSPvdAIhALSx
2Ky+mUAol0I2FdkaeUr3r5AObHsr5cH4OdFszF6K
-----END CERTIFICATE-----
`)
	caPrivateKeyPEM := []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIOfniC62mKKWIuszxytrnSOZhSZNYhdisDwFcOFOXrAxoAoGCCqGSM49
AwEHoUQDQgAEjHZZK5F9z3w9EkHz7siQWMsGHSOp1BdbAu0dba1jD4ysUm2F/3eR
wMfXbaIBSyNnT+e9/glHQsUmYVLu5MskmA==
-----END EC PRIVATE KEY-----
`)
	caKeyPair, err := NewKeyPair(caCertPEm, caPrivateKeyPEM)
	if err != nil {
		t.Fatalf("unexpected error generating CA keypair: %v", err)
	}

	tests := []struct {
		name           string
		tpl            *x509.Certificate
		caKeyPair      *KeyPair
		opts           []KeyPairOpt
		wantErr        bool
		wantCommonName string
		wantIssuer     string
	}{
		{
			name: "Invalid CA",
			tpl: &x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					CommonName: "cert",
				},
				NotBefore:             time.Now(),
				NotAfter:              time.Now().Add(defaultCertLifetime),
				KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
				BasicConstraintsValid: true,
			},
			caKeyPair: &KeyPair{
				CertPEM: []byte("invalid-cert"),
				KeyPEM:  []byte("invalid-key"),
			},
			wantErr:        true,
			wantCommonName: "",
			wantIssuer:     "",
		},
		{
			name: "Self-signed CA",
			tpl: &x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					CommonName: "ca",
				},
				NotBefore:             time.Now(),
				NotAfter:              time.Now().Add(-defaultCertLifetime), // Invalid NotAfter
				KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
				BasicConstraintsValid: true,
				IsCA:                  true,
			},
			caKeyPair:      nil,
			wantErr:        false,
			wantCommonName: "ca",
			wantIssuer:     "ca",
		},
		{
			name: "Leaf certificate",
			tpl: &x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					CommonName: "cert",
				},
				NotBefore:             time.Now(),
				NotAfter:              time.Now().Add(defaultCertLifetime),
				KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
				BasicConstraintsValid: true,
			},
			caKeyPair:      caKeyPair,
			wantErr:        false,
			wantCommonName: "cert",
			wantIssuer:     "mariadb-operator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyPair, err := NewKeyPairFromTemplate(tt.tpl, tt.caKeyPair)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyPairFromTemplate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			certs, err := keyPair.Certificates()
			if err != nil {
				t.Errorf("error getting certificates: %v", err)
			}
			cert := certs[0]

			if cert.Subject.CommonName != tt.wantCommonName {
				t.Errorf("CommonName = %v, want %v", cert.Subject.CommonName, tt.wantCommonName)
			}
			if cert.Issuer.CommonName != tt.wantIssuer {
				t.Errorf("Issuer = %v, want %v", cert.Issuer.CommonName, tt.wantIssuer)
			}
		})
	}
}

func TestUpdateSecret(t *testing.T) {
	tests := []struct {
		name          string
		keyPair       *KeyPair
		secret        *corev1.Secret
		certKey       string
		privateKeyKey string
		want          map[string][]byte
	}{
		{
			name: "Update empty secret",
			keyPair: &KeyPair{
				CertPEM: []byte("cert"),
				KeyPEM:  []byte("key"),
			},
			secret: &corev1.Secret{
				Data: map[string][]byte{},
			},
			certKey:       TLSCertKey,
			privateKeyKey: TLSKeyKey,
			want: map[string][]byte{
				TLSCertKey: []byte("cert"),
				TLSKeyKey:  []byte("key"),
			},
		},
		{
			name: "Update existing secret",
			keyPair: &KeyPair{
				CertPEM: []byte("new-cert"),
				KeyPEM:  []byte("new-key"),
			},
			secret: &corev1.Secret{
				Data: map[string][]byte{
					TLSCertKey: []byte("old-cert"),
					TLSKeyKey:  []byte("old-key"),
				},
			},
			certKey:       TLSCertKey,
			privateKeyKey: TLSKeyKey,
			want: map[string][]byte{
				TLSCertKey: []byte("new-cert"),
				TLSKeyKey:  []byte("new-key"),
			},
		},
		{
			name: "Update secret with other data",
			keyPair: &KeyPair{
				CertPEM: []byte("another-cert"),
				KeyPEM:  []byte("another-key"),
			},
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"other-key": []byte("other-value"),
				},
			},
			certKey:       TLSCertKey,
			privateKeyKey: TLSKeyKey,
			want: map[string][]byte{
				TLSCertKey:  []byte("another-cert"),
				TLSKeyKey:   []byte("another-key"),
				"other-key": []byte("other-value"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.keyPair.UpdateSecret(tt.secret, tt.certKey, tt.privateKeyKey)
			if !reflect.DeepEqual(tt.secret.Data, tt.want) {
				t.Errorf("UpdateSecret() = %v, want %v", tt.secret.Data, tt.want)
			}
		})
	}
}
