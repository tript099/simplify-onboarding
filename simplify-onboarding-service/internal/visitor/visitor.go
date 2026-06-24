// Package visitor implements anonymous identity capture and funnel events — the
// "capture identity early, verify late" principle. We know company-level/behaviour
// before anyone registers; personal identity is captured only at registration.
package visitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const cookieName = "visitor_id"

// Stage is the visitor's furthest point in the funnel.
type Stage string

const (
	StageAnonymous   Stage = "anonymous"
	StageTried       Stage = "tried_action"
	StageSawResult   Stage = "saw_result"
	StageHitWall     Stage = "hit_value_wall"
	StageRegistered  Stage = "registered"
	StageVerified    Stage = "verified"
)

// State is the snapshot returned to the frontend so it can resume the journey.
type State struct {
	VisitorID  string `json:"visitorId"`
	Stage      Stage  `json:"stage"`
	LastIntent string `json:"lastIntent,omitempty"`
	Source     string `json:"source,omitempty"`
}

// Service records anonymous sessions and events in Redis.
type Service struct {
	rdb    *redis.Client
	ttl    time.Duration
	secure bool
	log    *zap.Logger
}

// New builds the visitor service.
func New(rdb *redis.Client, ttl time.Duration, secure bool, log *zap.Logger) *Service {
	return &Service{rdb: rdb, ttl: ttl, secure: secure, log: log}
}

// EnsureCookie returns the existing visitor ID from the request, or mints a new
// one and sets the cookie on the response.
func (s *Service) EnsureCookie(w http.ResponseWriter, r *http.Request) (string, error) {
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		return c.Value, nil
	}
	id := uuid.NewString()
	st := State{VisitorID: id, Stage: StageAnonymous, Source: r.URL.Query().Get("utm_source")}
	if err := s.put(r.Context(), st); err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    id,
		Path:     "/",
		MaxAge:   int(s.ttl.Seconds()),
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return id, nil
}

// Record advances the funnel stage (monotonically) and stores the last intent.
func (s *Service) Record(ctx context.Context, visitorID, event, intent string) error {
	if visitorID == "" {
		return fmt.Errorf("missing visitor id")
	}
	st, err := s.get(ctx, visitorID)
	if err != nil {
		st = State{VisitorID: visitorID, Stage: StageAnonymous}
	}
	if stage, ok := eventStage(event); ok && rank(stage) > rank(st.Stage) {
		st.Stage = stage
	}
	if intent != "" {
		st.LastIntent = intent
	}
	if err := s.put(ctx, st); err != nil {
		return err
	}
	// Append to an event stream for later analytics export (capped TTL).
	entry, _ := json.Marshal(map[string]any{"event": event, "intent": intent, "at": time.Now().UTC()})
	streamKey := "visitor_events:" + visitorID
	_ = s.rdb.RPush(ctx, streamKey, entry).Err()
	_ = s.rdb.Expire(ctx, streamKey, s.ttl).Err()
	return nil
}

// Get returns the current state for a visitor.
func (s *Service) Get(ctx context.Context, visitorID string) (State, error) {
	return s.get(ctx, visitorID)
}

func (s *Service) put(ctx context.Context, st State) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("marshal visitor: %w", err)
	}
	return s.rdb.SetEx(ctx, key(st.VisitorID), raw, s.ttl).Err()
}

func (s *Service) get(ctx context.Context, id string) (State, error) {
	raw, err := s.rdb.Get(ctx, key(id)).Bytes()
	if err != nil {
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(raw, &st); err != nil {
		return State{}, err
	}
	return st, nil
}

func key(id string) string { return "visitor:" + id }

func eventStage(event string) (Stage, bool) {
	switch event {
	case "tried_action":
		return StageTried, true
	case "saw_result":
		return StageSawResult, true
	case "hit_value_wall":
		return StageHitWall, true
	case "registered":
		return StageRegistered, true
	case "verified":
		return StageVerified, true
	default:
		return "", false
	}
}

func rank(s Stage) int {
	order := map[Stage]int{
		StageAnonymous:  0,
		StageTried:      1,
		StageSawResult:  2,
		StageHitWall:    3,
		StageRegistered: 4,
		StageVerified:   5,
	}
	return order[s]
}
