package op

import "testing"

func TestParseRef(t *testing.T) {
	tests := []struct {
		ref     string
		wantErr bool
		vault   string
		item    string
		field   string
	}{
		// 正常系
		{"op://vault/item", false, "vault", "item", ""},
		{"op://vault/item/field", false, "vault", "item", "field"},
		{"op://Work/GitHub/token", false, "Work", "GitHub", "token"},
		// field なし（3セグメント目省略）
		{"op://My Vault/My Item", false, "My Vault", "My Item", ""},
		// 異常系
		{"not-a-ref", true, "", "", ""},
		{"http://vault/item", true, "", "", ""},
		{"op://", true, "", "", ""},
		{"op://vault-only", true, "", "", ""},
		{"op:///item", true, "", "", ""},  // vault が空
		{"op://vault/", true, "", "", ""}, // item が空
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got, err := ParseRef(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseRef(%q) err=%v, wantErr=%v", tt.ref, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Vault != tt.vault {
				t.Errorf("Vault=%q, want %q", got.Vault, tt.vault)
			}
			if got.Item != tt.item {
				t.Errorf("Item=%q, want %q", got.Item, tt.item)
			}
			if got.Field != tt.field {
				t.Errorf("Field=%q, want %q", got.Field, tt.field)
			}
		})
	}
}
