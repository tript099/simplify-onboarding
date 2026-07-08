// Package coreclient reads the public product catalog from Simplify Core. Core is the
// source of truth for the product registry (names, descriptions, launch URLs, logos)
// and the billing plans behind each product. We consume only the public catalog.
package coreclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Product is the subset of a Core catalog product the onboarding registry uses.
type Product struct {
	ID          string `json:"id"` // Core product UUID — used to deep-link to the purchase page
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Tagline     string `json:"tagline"`
	LogoURL     string `json:"logo_url"`
	WebsiteURL  string `json:"website_url"` // the product's launch URL
	Status      string `json:"status"`
}

type catalogResponse struct {
	AppID    string    `json:"app_id"`
	Products []Product `json:"products"`
}

// Client fetches Core's public master catalog for a single app_id.
type Client struct {
	base  string
	appID string
	token string
	http  *http.Client
	log   *zap.Logger
}

// New builds the client. baseURL like https://api-dev.simplifyaipro.com; appID is the
// master catalog id (e.g. "simplify-hiring"). Returns a disabled client if either is empty.
func New(baseURL, appID, token string, log *zap.Logger) *Client {
	return &Client{
		base:  strings.TrimRight(baseURL, "/"),
		appID: appID,
		token: token,
		http:  &http.Client{Timeout: 8 * time.Second},
		log:   log,
	}
}

// Enabled reports whether the client is configured to call Core.
func (c *Client) Enabled() bool { return c.base != "" && c.appID != "" }

// Fetch returns the products in Core's master catalog.
func (c *Client) Fetch(ctx context.Context) ([]Product, error) {
	endpoint := fmt.Sprintf("%s/api/v1/public/catalog?app_id=%s", c.base, url.QueryEscape(c.appID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("core catalog request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("core catalog: status %d", resp.StatusCode)
	}
	var out catalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("core catalog decode: %w", err)
	}
	return out.Products, nil
}
