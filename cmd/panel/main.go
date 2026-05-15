// Package main is the panel binary entrypoint. The build is intentionally
// minimal: load config, hand off to app.Build, install signal handler.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/KazuhaHub/passwall-sub-panel/internal/app"
	"github.com/KazuhaHub/passwall-sub-panel/internal/config"
)

func ensureDirs(cfg *config.Config) {
	for _, d := range []string{cfg.ConfigDir, cfg.DataDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			log.Fatalf("create directory %s: %v", d, err)
		}
	}
}

const defaultConfigPath = "config.yaml"

func main() {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	cfgPath := configPath()
	cfg, err := config.LoadOrGenerate(cfgPath)
	if err != nil {
		log.Fatalf("load config %s: %v", cfgPath, err)
	}
	ensureDirs(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a, err := app.Build(ctx, cfg)
	if err != nil {
		log.Fatalf("build app: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Passwall-Sub-Panel listening on %s", cfg.Listen)
		if err := a.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-quit:
		log.Printf("got signal %s, shutting down...", sig)
	case err := <-errCh:
		log.Printf("server error: %v, shutting down...", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := a.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func configPath() string {
	cfgPath := flag.String("config", "", "main config path")
	flag.Parse()
	if *cfgPath != "" {
		return *cfgPath
	}
	if v := os.Getenv("PSP_CONFIG"); v != "" {
		return v
	}
	return defaultConfigPath
}
