package v1alpha1

import (
	"testing"

	"k8s.io/utils/ptr"
)

func TestExternalMariaDB_IsTLSMutual(t *testing.T) {
	tests := []struct {
		name     string
		emdb     *ExternalMariaDB
		expected bool
	}{
		{
			name: "TLS disabled",
			emdb: &ExternalMariaDB{
				Spec: ExternalMariaDBSpec{
					TLS: &ExternalTLS{
						TLS: TLS{
							Enabled: false,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "TLS enabled",
			emdb: &ExternalMariaDB{
				Spec: ExternalMariaDBSpec{
					TLS: &ExternalTLS{
						TLS: TLS{
							Enabled: true,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "TLS enabled with mutual true",
			emdb: &ExternalMariaDB{
				Spec: ExternalMariaDBSpec{
					TLS: &ExternalTLS{
						TLS: TLS{
							Enabled: true,
						},
						Mutual: ptr.To(true),
					},
				},
			},
			expected: true,
		},
		{
			name: "TLS enabled with mutual false",
			emdb: &ExternalMariaDB{
				Spec: ExternalMariaDBSpec{
					TLS: &ExternalTLS{
						TLS: TLS{
							Enabled: true,
						},
						Mutual: ptr.To(false),
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.emdb.IsTLSMutual()
			if result != tt.expected {
				t.Errorf("IsTLSMutual() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
