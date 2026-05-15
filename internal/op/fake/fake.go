package fake

import (
	"context"
	"fmt"
)

type Client struct {
	Secrets    map[string]string // ref → value
	Titles     map[string]string // ref → title
	ResolveErr error             // nil 以外で全 ref に適用
}

func New() *Client {
	return &Client{
		Secrets: make(map[string]string),
		Titles:  make(map[string]string),
	}
}

func (c *Client) Resolve(ctx context.Context, ref string) (string, error) {
	if c.ResolveErr != nil {
		return "", c.ResolveErr
	}
	v, ok := c.Secrets[ref]
	if !ok {
		return "", fmt.Errorf("ref not found: %s", ref)
	}
	return v, nil
}

func (c *Client) ItemTitle(ctx context.Context, ref string) (string, error) {
	title, ok := c.Titles[ref]
	if !ok {
		return "", fmt.Errorf("title not found: %s", ref)
	}
	return title, nil
}
