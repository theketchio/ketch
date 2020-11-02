package v1beta1

import (
	"reflect"
	"testing"
)

func TestNewExposedPort(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		want    *ExposedPort
		wantErr bool
	}{
		{
			name: "properly formatted",
			port: "888/udp",
			want: &ExposedPort{
				Port:     888,
				Protocol: "UDP",
			},
		},
		{
			name:    "wrong format",
			port:    "888-udp",
			wantErr: true,
		},
		{
			name:    "bad port number",
			port:    "abc/udp",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewExposedPort(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewExposedPort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewExposedPort() got = %v, want %v", got, tt.want)
			}
		})
	}
}
