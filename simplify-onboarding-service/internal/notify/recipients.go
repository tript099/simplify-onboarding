package notify

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"
)

// recipientConfig is the JSON shape of the team-notification routing file.
type recipientConfig struct {
	Default []string `json:"default"`
	Rules   []struct {
		Type       string   `json:"type"`    // "" = any request type
		Product    string   `json:"product"` // "" = any product
		Recipients []string `json:"recipients"`
	} `json:"rules"`
}

// recipientsStore loads + caches the recipients file, reloading when it changes on
// disk (so edits take effect without a restart — and a UI can rewrite it later).
type recipientsStore struct {
	path string

	mu       sync.RWMutex
	cfg      recipientConfig
	modTime  time.Time
	loadedOK bool
}

func newRecipientsStore(path string) *recipientsStore {
	s := &recipientsStore{path: path}
	s.reloadIfChanged()
	return s
}

func (s *recipientsStore) reloadIfChanged() {
	info, err := os.Stat(s.path)
	if err != nil {
		return
	}
	s.mu.RLock()
	unchanged := s.loadedOK && info.ModTime().Equal(s.modTime)
	s.mu.RUnlock()
	if unchanged {
		return
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var cfg recipientConfig
	if json.Unmarshal(raw, &cfg) != nil {
		return
	}
	s.mu.Lock()
	s.cfg = cfg
	s.modTime = info.ModTime()
	s.loadedOK = true
	s.mu.Unlock()
}

// resolve returns the de-duplicated team recipients for a request type + product:
// the defaults plus any rule whose (type, product) filters match.
func (s *recipientsStore) resolve(reqType, product string) []string {
	s.reloadIfChanged()
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := map[string]bool{}
	out := []string{}
	add := func(list []string) {
		for _, e := range list {
			e = strings.TrimSpace(e)
			if e == "" {
				continue
			}
			k := strings.ToLower(e)
			if !seen[k] {
				seen[k] = true
				out = append(out, e)
			}
		}
	}
	add(s.cfg.Default)
	for _, r := range s.cfg.Rules {
		if (r.Type == "" || strings.EqualFold(r.Type, reqType)) &&
			(r.Product == "" || strings.EqualFold(r.Product, product)) {
			add(r.Recipients)
		}
	}
	return out
}
