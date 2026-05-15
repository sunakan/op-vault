package keychain

type Entry struct {
	Ref     string `json:"ref"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	Account string `json:"account"`
}
