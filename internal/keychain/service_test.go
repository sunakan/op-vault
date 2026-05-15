package keychain

import "testing"

func TestService(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{
			ref:  "op://Test/test02/password",
			want: "op-keychain:7f42d5946ebdc57e0d4eba7083aa752be1771a7b2e86f25fc42543c21444a9e1",
		},
	}
	for _, tt := range tests {
		got := Service(tt.ref)
		if got != tt.want {
			t.Errorf("Service(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}
