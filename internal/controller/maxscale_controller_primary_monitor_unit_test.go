package controller

import (
	"testing"

	mxsclient "github.com/mariadb-operator/mariadb-operator/v26/pkg/maxscale/client"
	"k8s.io/utils/ptr"
)

func TestIsPrimaryMonitor(t *testing.T) {
	monitor := func(primary *bool) *mxsclient.Data[*mxsclient.MonitorAttributes] {
		return &mxsclient.Data[*mxsclient.MonitorAttributes]{
			Attributes: &mxsclient.MonitorAttributes{
				Diagnostics: &mxsclient.MonitorDiagnostics{Primary: primary},
			},
		}
	}
	tests := []struct {
		name    string
		monitor *mxsclient.Data[*mxsclient.MonitorAttributes]
		want    bool
	}{
		{name: "single MaxScale without cooperative locks reports no primary flag", monitor: monitor(nil), want: true},
		{name: "cooperative monitor holding the majority", monitor: monitor(ptr.To(true)), want: true},
		{name: "cooperative monitor without the majority", monitor: monitor(ptr.To(false)), want: false},
		{
			name:    "monitor without diagnostics",
			monitor: &mxsclient.Data[*mxsclient.MonitorAttributes]{Attributes: &mxsclient.MonitorAttributes{}},
			want:    false,
		},
		{name: "missing monitor", monitor: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPrimaryMonitor(tt.monitor); got != tt.want {
				t.Fatalf("unexpected result: got %t, want %t", got, tt.want)
			}
		})
	}
}
