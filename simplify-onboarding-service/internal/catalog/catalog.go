// Package catalog is the product registry: the eight Simplify products, their
// motion (self-serve / enterprise), trial scope and data residency. It drives
// the homepage cards, the "How will you use this?" split and CTA selection.
package catalog

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/simplify/onboarding/internal/coreclient"
	"go.uber.org/zap"
)

// Product is the public, motion-aware descriptor for one Simplify product.
type Product struct {
	Key            string   `json:"key"`
	Intent         string   `json:"intent"` // problem-first label
	Name           string   `json:"name"`
	Tagline        string   `json:"tagline"`
	TrialScope     string   `json:"trialScope"`
	AllowedTypes   []string `json:"allowedUserTypes"`
	AsksUserType   bool     `json:"asksUserType"`
	EnterpriseOnly bool     `json:"enterpriseOnly"`
	DataResidency  []string `json:"dataResidency"`
	// From Simplify Core (overlaid at runtime): the product's launch URL + logo.
	LaunchURL string `json:"launchUrl,omitempty"`
	LogoURL   string `json:"logoUrl,omitempty"`
}

// coreAlias maps a Core catalog slug to our local product key so Core data overlays the
// right built-in (preserving its icon/accent/intent on the frontend). Slugs not listed
// are added as new products under a normalized key.
var coreAlias = map[string]string{
	"simplify-docflow": "docflow",
	"insights":         "insights",
	"talent":           "talent",
	"hiring-platform":  "hiring",
	"simplify-hiring":  "hiring",
	"Simplify_HR":      "hr",
}

// Catalog holds the registry and answers lookups. When a Core client is configured it
// overlays Core's names/taglines/launch URLs/logos (refreshed in the background); Core
// being empty or unreachable always falls back to the built-in list.
type Catalog struct {
	mu       sync.RWMutex
	products []Product
	byKey    map[string]Product

	builtins []Product
	core     coreFetcher
	log      *zap.Logger
}

// coreFetcher is the slice of coreclient.Client the catalog needs (kept as an interface
// so catalog has no hard dependency and stays easy to test).
type coreFetcher interface {
	Enabled() bool
	Fetch(ctx context.Context) ([]coreclient.Product, error)
}

var residency = []string{"ID", "SG", "IN", "AE"}

// builtins mirrors the eight products configured in the frontend.
var builtins = []Product{
	{Key: "legal", Intent: "Review a legal document", Name: "SimplifyLegal", Tagline: "Legal chatbot, AI Lawyer, document review", TrialScope: "Legal chatbot, AI Lawyer, and document review", AllowedTypes: []string{"enterprise", "self_serve"}, AsksUserType: true, EnterpriseOnly: false, DataResidency: residency},
	{Key: "docflow", Intent: "Automate document processing", Name: "SimplifyDocFlow", Tagline: "OCR, extraction and document workflows", TrialScope: "OCR a single document; access to SimplifyDrive", AllowedTypes: []string{"enterprise", "self_serve"}, AsksUserType: true, EnterpriseOnly: false, DataResidency: residency},
	{Key: "insights", Intent: "Generate business insights", Name: "SimplifyInsights", Tagline: "Ask business questions across your data", TrialScope: "Access to data for 2 selected companies", AllowedTypes: []string{"enterprise", "self_serve"}, AsksUserType: true, EnterpriseOnly: false, DataResidency: residency},
	{Key: "hiring", Intent: "Hire talent faster", Name: "SimplifyHiring", Tagline: "JD creation, resume assessment, AI interviews", TrialScope: "One full hiring cycle: JD → publish → assess → AI interview", AllowedTypes: []string{"enterprise", "vendor", "candidate"}, AsksUserType: false, EnterpriseOnly: false, DataResidency: residency},
	{Key: "studio", Intent: "Build software faster", Name: "SimplifyStudio", Tagline: "From a prompt to a working build", TrialScope: "Create use cases and a PRD", AllowedTypes: []string{"enterprise", "self_serve"}, AsksUserType: true, EnterpriseOnly: false, DataResidency: residency},
	{Key: "transformer", Intent: "Modernize legacy systems", Name: "SimplifyTransformer", Tagline: "Any-to-any legacy modernization AI", TrialScope: "Sample legacy snippet assessment (scoped preview)", AllowedTypes: []string{"enterprise"}, AsksUserType: false, EnterpriseOnly: true, DataResidency: residency},
	{Key: "talent", Intent: "Assess skills and careers", Name: "SimplifyTalent", Tagline: "Assessments, reports and learning paths", TrialScope: "One assessment end-to-end, with report", AllowedTypes: []string{"enterprise", "self_serve"}, AsksUserType: true, EnterpriseOnly: false, DataResidency: residency},
	{Key: "credit", Intent: "Assess credit risk", Name: "SimplifyCredit", Tagline: "Credit analysis and risk scoring", TrialScope: "Credit analysis of self or 1 company", AllowedTypes: []string{"enterprise", "self_serve"}, AsksUserType: true, EnterpriseOnly: false, DataResidency: residency},
}

// New builds the catalog, optionally overriding the built-ins from a JSON file. When
// core is enabled, Core's catalog is fetched once now and then refreshed in the
// background every 5 minutes; List/Get always serve the latest good snapshot.
func New(file string, core coreFetcher, log *zap.Logger) *Catalog {
	products := builtins
	if file != "" {
		if raw, err := os.ReadFile(file); err != nil {
			log.Warn("product registry file unreadable, using built-ins", zap.String("file", file), zap.Error(err))
		} else {
			var loaded []Product
			if err := json.Unmarshal(raw, &loaded); err != nil {
				log.Warn("product registry file invalid, using built-ins", zap.Error(err))
			} else if len(loaded) > 0 {
				products = loaded
			}
		}
	}
	c := &Catalog{builtins: products, core: core, log: log}
	c.set(products)
	if core != nil && core.Enabled() {
		c.refresh()   // best-effort initial fetch (falls back to built-ins on error)
		go c.loop()   // keep it fresh
	}
	return c
}

// List returns all products (latest snapshot).
func (c *Catalog) List() []Product {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.products
}

// Get returns one product by key.
func (c *Catalog) Get(key string) (Product, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.byKey[key]
	return p, ok
}

func (c *Catalog) set(products []Product) {
	byKey := make(map[string]Product, len(products))
	for _, p := range products {
		byKey[p.Key] = p
	}
	c.mu.Lock()
	c.products = products
	c.byKey = byKey
	c.mu.Unlock()
}

func (c *Catalog) loop() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for range t.C {
		c.refresh()
	}
}

// refresh pulls Core's catalog and overlays it on the built-ins. On any error it leaves
// the current snapshot untouched — the home page never breaks because Core is down.
func (c *Catalog) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	coreProducts, err := c.core.Fetch(ctx)
	if err != nil {
		c.log.Warn("core catalog fetch failed — keeping current product list", zap.Error(err))
		return
	}
	merged := overlayCore(c.builtins, coreProducts)
	c.set(merged)
	c.log.Info("product catalog refreshed from Simplify Core", zap.Int("products", len(merged)))
}

// overlayCore returns the built-in list with Core's name/tagline/launchURL/logo overlaid
// (matched via coreAlias), plus any Core-only products appended. Empty Core fields never
// overwrite good built-in text.
func overlayCore(builtins []Product, core []coreclient.Product) []Product {
	out := make([]Product, len(builtins))
	copy(out, builtins)
	idx := make(map[string]int, len(out))
	for i, p := range out {
		idx[p.Key] = i
	}
	for _, cp := range core {
		if cp.Status != "" && cp.Status != "PUBLISHED" {
			continue
		}
		key := coreAlias[cp.Slug]
		if key == "" {
			key = normalizeKey(cp.Slug)
		}
		tagline := firstNonEmpty(cp.Tagline, cp.Description)
		if i, ok := idx[key]; ok {
			if cp.Name != "" {
				out[i].Name = cp.Name // Core name, as-is
			}
			if tagline != "" {
				out[i].Tagline = tagline
			}
			if cp.WebsiteURL != "" {
				out[i].LaunchURL = cp.WebsiteURL
			}
			if cp.LogoURL != "" {
				out[i].LogoURL = cp.LogoURL
			}
			continue
		}
		// Core-only product — append with safe defaults.
		out = append(out, Product{
			Key:           key,
			Name:          firstNonEmpty(cp.Name, cp.Slug),
			Intent:        firstNonEmpty(tagline, cp.Name, cp.Slug),
			Tagline:       tagline,
			LaunchURL:     cp.WebsiteURL,
			LogoURL:       cp.LogoURL,
			AllowedTypes:  []string{"enterprise", "self_serve"},
			AsksUserType:  true,
			DataResidency: residency,
		})
		idx[key] = len(out) - 1
	}
	return out
}

func normalizeKey(slug string) string {
	s := strings.ToLower(strings.TrimSpace(slug))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	return strings.TrimPrefix(s, "simplify-")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// PrimaryCTA picks the value-first primary CTA for a product + chosen motion.
func PrimaryCTA(p Product, teamChosen bool) string {
	switch {
	case p.EnterpriseOnly:
		return "request_poc"
	case teamChosen:
		return "request_demo"
	default:
		return "buy" // "Try it now" / Activate
	}
}
