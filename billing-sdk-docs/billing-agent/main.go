// billing-agent — the product-side "billing PDP" (the Cerbos of quota).
//
// Wiring only: config → authenticator (api key or Zitadel M2M) → upstream client
// → batcher (report write-back) → PDP → HTTP server. See internal/ for the pieces.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/simplify/billing-agent/internal/auth"
	"github.com/simplify/billing-agent/internal/batch"
	"github.com/simplify/billing-agent/internal/config"
	"github.com/simplify/billing-agent/internal/pdp"
	"github.com/simplify/billing-agent/internal/upstream"
)

func main() {
	cfg := config.Load()

	authr, err := auth.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	up := upstream.New(cfg.BaseURL, authr)
	agent := pdp.New(up, cfg.CacheTTL, cfg.FailOpen)

	// Batched consumption write-back for report() (free-quota ledger). authorize
	// stays synchronous. onFlush invalidates the flushed orgs' read cache.
	var batcher *batch.Batcher
	if cfg.FlushSize > 0 && cfg.FlushInterval > 0 {
		batcher = batch.New(up, cfg.FlushSize, cfg.FlushInterval, agent.Invalidate)
		agent.SetBatcher(batcher)
	}

	log.Printf("billing-agent on %s → %s (auth=%s, fail-mode=%s, cache=%s, batch=%dev/%s)",
		cfg.Listen, cfg.BaseURL, authr.Kind(), failMode(cfg.FailOpen), cfg.CacheTTL, cfg.FlushSize, cfg.FlushInterval)

	srv := &http.Server{
		Addr:         cfg.Listen,
		Handler:      agent.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// On SIGTERM/SIGINT, stop accepting new requests.
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
		<-stop
		log.Print("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	// Server stopped — flush buffered consumption SYNCHRONOUSLY before exit so no
	// usage is lost on deploy/restart. (Must run in main, not the signal goroutine,
	// or the process can exit before the flush completes.)
	if batcher != nil {
		log.Print("flushing buffered usage before exit")
		batcher.Close()
	}
}

func failMode(open bool) string {
	if open {
		return "open"
	}
	return "closed"
}
