//go:build darwin

package op

import (
	"context"
	"errors"
	"testing"

	onepassword "github.com/1password/onepassword-sdk-go"
)

type fakeSecretsAPI struct {
	onepassword.SecretsAPI
	value string
	refs  []string
}

func (f *fakeSecretsAPI) Resolve(_ context.Context, ref string) (string, error) {
	f.refs = append(f.refs, ref)
	return f.value, nil
}

type fakeVaultsAPI struct {
	onepassword.VaultsAPI
	vaults []onepassword.VaultOverview
	err    error
	calls  int
}

func (f *fakeVaultsAPI) List(_ context.Context, _ ...onepassword.VaultListParams) ([]onepassword.VaultOverview, error) {
	f.calls++
	return f.vaults, f.err
}

type fakeItemsAPI struct {
	onepassword.ItemsAPI
	items    []onepassword.ItemOverview
	err      error
	vaultIDs []string
}

func (f *fakeItemsAPI) List(_ context.Context, vaultID string, _ ...onepassword.ItemListFilter) ([]onepassword.ItemOverview, error) {
	f.vaultIDs = append(f.vaultIDs, vaultID)
	return f.items, f.err
}

type fakeClient struct {
	secrets onepassword.SecretsAPI
	vaults  onepassword.VaultsAPI
	items   onepassword.ItemsAPI
}

func (f *fakeClient) Secrets() onepassword.SecretsAPI { return f.secrets }
func (f *fakeClient) Vaults() onepassword.VaultsAPI   { return f.vaults }
func (f *fakeClient) Items() onepassword.ItemsAPI     { return f.items }

func TestResolveWithClient(t *testing.T) {
	t.Run("custom field uses the standard secret resolver", func(t *testing.T) {
		secrets := &fakeSecretsAPI{value: "custom-value"}
		vaults := &fakeVaultsAPI{}
		client := &fakeClient{secrets: secrets, vaults: vaults}

		got, err := resolveWithClient(context.Background(), client, "op://Test/ExistedItem/a1")
		if err != nil {
			t.Fatalf("resolveWithClient: %v", err)
		}
		if got != "custom-value" {
			t.Fatalf("got %q, want custom-value", got)
		}
		if len(secrets.refs) != 1 || secrets.refs[0] != "op://Test/ExistedItem/a1" {
			t.Fatalf("secret resolver refs = %v", secrets.refs)
		}
		if vaults.calls != 0 {
			t.Fatalf("vault list calls = %d, want 0", vaults.calls)
		}
	})

	t.Run("website inside a section remains a custom field", func(t *testing.T) {
		secrets := &fakeSecretsAPI{value: "section-value"}
		vaults := &fakeVaultsAPI{}
		client := &fakeClient{secrets: secrets, vaults: vaults}

		got, err := resolveWithClient(context.Background(), client, "op://Test/ExistedItem/Section/website")
		if err != nil {
			t.Fatalf("resolveWithClient: %v", err)
		}
		if got != "section-value" {
			t.Fatalf("got %q, want section-value", got)
		}
		if vaults.calls != 0 {
			t.Fatalf("vault list calls = %d, want 0", vaults.calls)
		}
	})

	t.Run("primary website uses the first website", func(t *testing.T) {
		vaults := &fakeVaultsAPI{vaults: []onepassword.VaultOverview{
			{ID: "vault-id", Title: "Test"},
		}}
		items := &fakeItemsAPI{items: []onepassword.ItemOverview{
			{
				ID:      "item-id",
				Title:   "ExistedItem",
				VaultID: "vault-id",
				Websites: []onepassword.Website{
					{URL: "https://example.com/test", Label: "website"},
					{URL: "https://example.com/test2", Label: "website"},
				},
			},
		}}
		client := &fakeClient{vaults: vaults, items: items}

		got, err := resolveWithClient(context.Background(), client, "op://test/existeditem/WEBSITE")
		if err != nil {
			t.Fatalf("resolveWithClient: %v", err)
		}
		if got != "https://example.com/test" {
			t.Fatalf("got %q, want primary website", got)
		}
		if len(items.vaultIDs) != 1 || items.vaultIDs[0] != "vault-id" {
			t.Fatalf("item list vault IDs = %v", items.vaultIDs)
		}
	})

	t.Run("vault and item IDs are accepted", func(t *testing.T) {
		vaults := &fakeVaultsAPI{vaults: []onepassword.VaultOverview{
			{ID: "vault-id", Title: "Test"},
		}}
		items := &fakeItemsAPI{items: []onepassword.ItemOverview{
			{ID: "item-id", Title: "ExistedItem", Websites: []onepassword.Website{{URL: "https://example.com/test"}}},
		}}
		client := &fakeClient{vaults: vaults, items: items}

		got, err := resolveWithClient(context.Background(), client, "op://vault-id/item-id/website")
		if err != nil {
			t.Fatalf("resolveWithClient: %v", err)
		}
		if got != "https://example.com/test" {
			t.Fatalf("got %q, want primary website", got)
		}
	})

	t.Run("missing website is not found", func(t *testing.T) {
		vaults := &fakeVaultsAPI{vaults: []onepassword.VaultOverview{{ID: "vault-id", Title: "Test"}}}
		items := &fakeItemsAPI{items: []onepassword.ItemOverview{{ID: "item-id", Title: "ExistedItem"}}}
		client := &fakeClient{vaults: vaults, items: items}

		_, err := resolveWithClient(context.Background(), client, "op://Test/ExistedItem/website")
		if err == nil || err.Error() != "not found in 1Password: op://Test/ExistedItem/website" {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("duplicate item titles are rejected", func(t *testing.T) {
		vaults := &fakeVaultsAPI{vaults: []onepassword.VaultOverview{{ID: "vault-id", Title: "Test"}}}
		items := &fakeItemsAPI{items: []onepassword.ItemOverview{
			{ID: "item-1", Title: "ExistedItem", Websites: []onepassword.Website{{URL: "https://one.example"}}},
			{ID: "item-2", Title: "ExistedItem", Websites: []onepassword.Website{{URL: "https://two.example"}}},
		}}
		client := &fakeClient{vaults: vaults, items: items}

		_, err := resolveWithClient(context.Background(), client, "op://Test/ExistedItem/website")
		if err == nil || err.Error() != "more than one item matched in 1Password: op://Test/ExistedItem/website" {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("SDK listing errors are preserved", func(t *testing.T) {
		wantErr := errors.New("list vaults failed")
		client := &fakeClient{vaults: &fakeVaultsAPI{err: wantErr}}

		_, err := resolveWithClient(context.Background(), client, "op://Test/ExistedItem/website")
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})
}
