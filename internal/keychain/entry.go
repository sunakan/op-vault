//go:build darwin

package keychain

// Entry represents a cached 1Password secret.
type Entry struct {
	Ref      string `json:"ref"`
	ItemName string `json:"item_name"`
	Value    string `json:"value"`
}
