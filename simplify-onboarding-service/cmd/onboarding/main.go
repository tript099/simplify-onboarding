// simplify-onboarding-service
// ============================
// Identity + value-first onboarding backend for the Simplify suite.
//
// Owns: anonymous identity capture, branded registration, email/mobile OTP,
// product registry + motion, entitlement reads (SimplifyCore), and the session
// that single sign-on is built on. See ../SIMPLIFY_ONBOARDING_BACKEND_PLAN.md.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/simplify/onboarding/internal/config"
	"github.com/simplify/onboarding/internal/server"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()

	log, err := buildLogger(cfg.AppEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	log.Info("simplify-onboarding-service starting",
		zap.String("port", cfg.Port),
		zap.String("env", cfg.AppEnv),
		zap.String("identity_provider", cfg.IdentityProvider),
	)

	srv, err := server.New(cfg, log)
	if err != nil {
		log.Fatal("failed to build server", zap.Error(err))
	}

	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Router(),
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server error", zap.Error(err))
		}
	}()
	log.Info("simplify-onboarding-service ready", zap.String("addr", httpSrv.Addr))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	}
	log.Info("simplify-onboarding-service stopped")
}

func buildLogger(env string) (*zap.Logger, error) {
	if env == "production" {
		return zap.NewProduction()
	}
	return zap.NewDevelopment()
}
