package pki

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("NewKeyPair", func() {
	DescribeTable("creates a keypair from PEMs",
		func(certPEM, keyPEM []byte, opts []KeyPairOpt, wantErr bool) {
			_, err := NewKeyPair(certPEM, keyPEM, opts...)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Empty Cert PEM",
			[]byte(""),
			[]byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIAdp3iKnNA1kO2Ep5Hw7owMcm06SecFGdOqW/vO4k2AjoAoGCCqGSM49
AwEHoUQDQgAEiTVhkriBksuWW5W3Mv9L918m1BECaHUl7ZV/Pz2q84wY9aEbxe2P
J3c22DtEFzg9emNuruVS5/HL+hanzz4o+g==
-----END EC PRIVATE KEY-----
`),
			[]KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeECDSA)},
			true,
		),
		Entry("Empty Key PEM",
			[]byte(`-----BEGIN CERTIFICATE-----
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
			[]byte(""),
			[]KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeECDSA)},
			true,
		),
		Entry("Invalid Cert PEM",
			[]byte("invalid-cert"),
			[]byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIAdp3iKnNA1kO2Ep5Hw7owMcm06SecFGdOqW/vO4k2AjoAoGCCqGSM49
AwEHoUQDQgAEiTVhkriBksuWW5W3Mv9L918m1BECaHUl7ZV/Pz2q84wY9aEbxe2P
J3c22DtEFzg9emNuruVS5/HL+hanzz4o+g==
-----END EC PRIVATE KEY-----
`),
			[]KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeECDSA)},
			true,
		),
		Entry("Invalid Key PEM",
			[]byte(`-----BEGIN CERTIFICATE-----
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
			[]byte("invalid-key"),
			[]KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeRSA)},
			true,
		),
		Entry("Valid",
			[]byte(`-----BEGIN CERTIFICATE-----
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
			[]byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIAdp3iKnNA1kO2Ep5Hw7owMcm06SecFGdOqW/vO4k2AjoAoGCCqGSM49
AwEHoUQDQgAEiTVhkriBksuWW5W3Mv9L918m1BECaHUl7ZV/Pz2q84wY9aEbxe2P
J3c22DtEFzg9emNuruVS5/HL+hanzz4o+g==
-----END EC PRIVATE KEY-----
`),
			[]KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeECDSA)},
			false,
		),
		Entry("Unmatched",
			[]byte(`-----BEGIN CERTIFICATE-----
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
			[]byte(`-----BEGIN RSA PRIVATE KEY-----
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
			[]KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeRSA)},
			true,
		),
		Entry("Unsupported",
			[]byte(`-----BEGIN CERTIFICATE-----
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
			[]byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIAdp3iKnNA1kO2Ep5Hw7owMcm06SecFGdOqW/vO4k2AjoAoGCCqGSM49
AwEHoUQDQgAEiTVhkriBksuWW5W3Mv9L918m1BECaHUl7ZV/Pz2q84wY9aEbxe2P
J3c22DtEFzg9emNuruVS5/HL+hanzz4o+g==
-----END EC PRIVATE KEY-----
`),
			[]KeyPairOpt{WithSupportedPrivateKeys(PrivateKeyTypeRSA)},
			true,
		),
	)
})

var _ = Describe("NewKeyPairFromSecret", func() {
	DescribeTable("creates a keypair from a secret",
		func(secret *corev1.Secret, certKey, privateKeyKey string, opts []KeyPairOpt, wantErr bool) {
			_, err := NewKeyPairFromSecret(secret, certKey, privateKeyKey, opts...)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Empty Secret",
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Data: map[string][]byte{},
			},
			TLSCertKey,
			TLSKeyKey,
			[]KeyPairOpt(nil),
			true,
		),
		Entry("Missing Cert Key",
			&corev1.Secret{
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
			TLSCertKey,
			TLSKeyKey,
			[]KeyPairOpt(nil),
			true,
		),
		Entry("Missing Key Key",
			&corev1.Secret{
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
			TLSCertKey,
			TLSKeyKey,
			[]KeyPairOpt(nil),
			true,
		),
		Entry("Invalid Cert PEM",
			&corev1.Secret{
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
			TLSCertKey,
			TLSKeyKey,
			[]KeyPairOpt(nil),
			true,
		),
		Entry("Invalid Key PEM",
			&corev1.Secret{
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
			TLSCertKey,
			TLSKeyKey,
			[]KeyPairOpt(nil),
			true,
		),
		Entry("Valid",
			&corev1.Secret{
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
			TLSCertKey,
			TLSKeyKey,
			[]KeyPairOpt(nil),
			false,
		),
	)
})

var _ = Describe("NewKeyPairFromTemplate", func() {
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
	var caKeyPair *KeyPair
	BeforeEach(func() {
		var err error
		caKeyPair, err = NewKeyPair(caCertPEm, caPrivateKeyPEM)
		Expect(err).NotTo(HaveOccurred())
	})

	DescribeTable("creates a keypair from a template",
		func(tpl *x509.Certificate, caKeyPairFn func() *KeyPair, wantErr bool, wantCommonName, wantIssuer string) {
			keyPair, err := NewKeyPairFromTemplate(tpl, caKeyPairFn())
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if wantErr {
				return
			}

			cert, err := keyPair.LeafCertificate()
			Expect(err).NotTo(HaveOccurred())

			Expect(cert.Subject.CommonName).To(Equal(wantCommonName))
			Expect(cert.Issuer.CommonName).To(Equal(wantIssuer))
		},
		Entry("Invalid CA",
			&x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					CommonName: "cert",
				},
				NotBefore:             time.Now(),
				NotAfter:              time.Now().Add(DefaultCertLifetime),
				KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
				BasicConstraintsValid: true,
			},
			func() *KeyPair {
				return &KeyPair{
					CertPEM: []byte("invalid-cert"),
					KeyPEM:  []byte("invalid-key"),
				}
			},
			true,
			"",
			"",
		),
		Entry("Self-signed CA",
			&x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					CommonName: "ca",
				},
				NotBefore:             time.Now(),
				NotAfter:              time.Now().Add(-DefaultCertLifetime), // Invalid NotAfter
				KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
				BasicConstraintsValid: true,
				IsCA:                  true,
			},
			func() *KeyPair {
				return nil
			},
			false,
			"ca",
			"ca",
		),
		Entry("Leaf certificate",
			&x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					CommonName: "cert",
				},
				NotBefore:             time.Now(),
				NotAfter:              time.Now().Add(DefaultCertLifetime),
				KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
				BasicConstraintsValid: true,
			},
			func() *KeyPair {
				return caKeyPair
			},
			false,
			"cert",
			"mariadb-operator",
		),
	)
})

var _ = Describe("UpdateSecret", func() {
	DescribeTable("updates a secret with the keypair",
		func(keyPair *KeyPair, secret *corev1.Secret, certKey, privateKeyKey string, want map[string][]byte) {
			keyPair.UpdateSecret(secret, certKey, privateKeyKey)
			Expect(secret.Data).To(Equal(want))
		},
		Entry("Update empty secret",
			&KeyPair{
				CertPEM: []byte("cert"),
				KeyPEM:  []byte("key"),
			},
			&corev1.Secret{
				Data: map[string][]byte{},
			},
			TLSCertKey,
			TLSKeyKey,
			map[string][]byte{
				TLSCertKey: []byte("cert"),
				TLSKeyKey:  []byte("key"),
			},
		),
		Entry("Update existing secret",
			&KeyPair{
				CertPEM: []byte("new-cert"),
				KeyPEM:  []byte("new-key"),
			},
			&corev1.Secret{
				Data: map[string][]byte{
					TLSCertKey: []byte("old-cert"),
					TLSKeyKey:  []byte("old-key"),
				},
			},
			TLSCertKey,
			TLSKeyKey,
			map[string][]byte{
				TLSCertKey: []byte("new-cert"),
				TLSKeyKey:  []byte("new-key"),
			},
		),
		Entry("Update secret with other data",
			&KeyPair{
				CertPEM: []byte("another-cert"),
				KeyPEM:  []byte("another-key"),
			},
			&corev1.Secret{
				Data: map[string][]byte{
					"other-key": []byte("other-value"),
				},
			},
			TLSCertKey,
			TLSKeyKey,
			map[string][]byte{
				TLSCertKey:  []byte("another-cert"),
				TLSKeyKey:   []byte("another-key"),
				"other-key": []byte("other-value"),
			},
		),
	)
})
