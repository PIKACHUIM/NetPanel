package handlers

import (
	"errors"
	"testing"
)

func TestFriendlyPortForwardError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "windows address in use",
			err:  errors.New("bind: Only one usage of each socket address is normally permitted."),
			want: "监听端口已被占用，请更换端口后重试",
		},
		{
			name: "unix address in use",
			err:  errors.New("listen tcp: address already in use"),
			want: "监听端口已被占用，请更换端口后重试",
		},
		{
			name: "other error retained",
			err:  errors.New("invalid listener configuration"),
			want: "invalid listener configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := friendlyPortForwardError(tt.err); got != tt.want {
				t.Fatalf("friendlyPortForwardError() = %q, want %q", got, tt.want)
			}
		})
	}
}
