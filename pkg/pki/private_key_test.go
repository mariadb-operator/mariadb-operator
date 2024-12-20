package pki

import (
	"crypto/ecdsa"
	"testing"
)

func TestPrivateKey(t *testing.T) {
	privateKey, err := GeneratePrivateKey()
	if err != nil {
		t.Fatalf("unexpected error generating private key: %v", err)
	}

	bytes, err := MarshalPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("unexpected error marshaling private key: %v", err)
	}

	pemKey, err := pemEncodePrivateKey(bytes, privateKey)
	if err != nil {
		t.Fatalf("unexpected error encoding private key: %v", err)
	}

	parsedKey, err := ParsePrivateKey(pemKey, []PrivateKey{PrivateKeyTypeECDSA})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := parsedKey.(*ecdsa.PrivateKey); !ok {
		t.Fatalf("expected *ecdsa.PrivateKey, got %T", parsedKey)
	}
}

func TestParsePrivateKey(t *testing.T) {
	tests := []struct {
		name     string
		pemKey   []byte
		keyTypes []PrivateKey
		wantErr  bool
	}{
		{
			name:    "Invalid",
			pemKey:  []byte("invalid"),
			wantErr: true,
		},
		{
			name: "Unsupported",
			pemKey: []byte(`-----BEGIN RSA PRIVATE KEY-----
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
			wantErr: true,
		},
		{
			name: "Valid",
			pemKey: []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIAdp3iKnNA1kO2Ep5Hw7owMcm06SecFGdOqW/vO4k2AjoAoGCCqGSM49
AwEHoUQDQgAEiTVhkriBksuWW5W3Mv9L918m1BECaHUl7ZV/Pz2q84wY9aEbxe2P
J3c22DtEFzg9emNuruVS5/HL+hanzz4o+g==
-----END EC PRIVATE KEY-----
`),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedKey, err := ParsePrivateKey(tt.pemKey, []PrivateKey{PrivateKeyTypeECDSA})
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error: %v, got: %v", tt.wantErr, err)
			}
			if !tt.wantErr {
				if _, ok := parsedKey.(*ecdsa.PrivateKey); !ok {
					t.Fatalf("expected *ecdsa.PrivateKey, got %T", parsedKey)
				}
			}
		})
	}
}
