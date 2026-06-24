// Package catalog is the product registry: the eight Simplify products, their
// motion (self-serve / enterprise), trial scope and data residency. It drives
// the homepage cards, the "How will you use this?" split and CTA selection.
package catalog

import (
	"encoding/json"
	"os"

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
}

// Catalog holds the registry and answers lookups.
type Catalog struct {
	products []Product
	byKey    map[string]Product
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

// New builds the catalog, optionally overriding the built-ins from a JSON file.
func New(file string, log *zap.Logger) *Catalog {
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
	byKey := make(map[string]Product, len(products))
	for _, p := range products {
		byKey[p.Key] = p
	}
	return &Catalog{products: products, byKey: byKey}
}

// List returns all products.
func (c *Catalog) List() []Product { return c.products }

// Get returns one product by key.
func (c *Catalog) Get(key string) (Product, bool) {
	p, ok := c.byKey[key]
	return p, ok
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
