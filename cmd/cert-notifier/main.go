package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/the100rabh/ssl-cert-notifier/internal/app"
	"github.com/the100rabh/ssl-cert-notifier/internal/checker"
	"github.com/the100rabh/ssl-cert-notifier/internal/config"
	yaml "gopkg.in/yaml.v3"
)

func main() {
	// Setup root context with signal cancellation for graceful shutdown (SIGINT, SIGTERM)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("FATAL: Failed to load configuration: %v", err)
	}
	yamlCfg, err := yaml.Marshal(cfg)
	if err != nil {
		log.Printf("WARNING: Could not marshal configuration to YAML for pretty printing: %v", err)
		log.Printf("Configuration loaded as follows: %+v", cfg) // Fallback to default print
	} else {
		log.Printf("Configuration loaded as follows:\n%s", string(yamlCfg))
	}

	// Initialize all configured notifiers
	initializedNotifiers, initErrors := app.InitializeNotifiers(cfg.Notifiers)

	// Check for initialization errors only for notifiers that are actually used by websites
	for _, site := range cfg.Websites {
		for _, notifierName := range site.Notifiers {
			if err, exists := initErrors[notifierName]; exists {
				log.Fatalf("FATAL: Notifier '%s' for website '%s' failed to initialize and cannot be used: %v", notifierName, site.URL, err)
			}
		}
	}
	log.Printf("Successfully initialized %d notifiers (some may have warnings, but no critical errors for used notifiers).", len(initializedNotifiers))

	// Get the check interval from settings
	checkInterval, err := cfg.Settings.GetCheckIntervalDuration()

	// Check if check_time is specified in settings
	if cfg.Settings.CheckTime != "" {
		nextRun, err := cfg.Settings.GetNextCheckTime(time.Now())
		if err != nil {
			log.Fatalf("FATAL: Invalid check_time: %v", err)
		}

		for {
			duration := time.Until(nextRun)
			select {
			case <-ctx.Done():
				log.Println("Received termination signal, shutting down cert-notifier gracefully.")
				return
			case <-time.After(duration):
			}

			log.Printf("Scheduled check at %s started...", cfg.Settings.CheckTime)
			app.RunChecks(ctx, cfg, initializedNotifiers, checker.Check)

			nextRun, err = cfg.Settings.GetNextCheckTime(time.Now())
			if err != nil {
				log.Fatalf("FATAL: Invalid check_time on recalculation: %v", err)
			}
			log.Printf("Checks complete. Next check scheduled for %s (check_time: %s).",
				nextRun.Format("2006-01-02 15:04:05"), cfg.Settings.CheckTime)
		}
	} else {
		if err != nil {
			log.Fatalf("FATAL: Invalid check_interval: %v", err)
		}
		if checkInterval == 0 {
			log.Println("check_interval is 0, running checks only once.")
			app.RunChecks(ctx, cfg, initializedNotifiers, checker.Check)
			return
		}

		// Run checks immediately on start, then on every tick
		log.Println("Performing initial check...")
		app.RunChecks(ctx, cfg, initializedNotifiers, checker.Check)

		log.Printf("Checks complete. Will run again in %s.", checkInterval)

		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("Received termination signal, shutting down cert-notifier gracefully.")
				return
			case <-ticker.C:
				log.Println("Scheduled check started...")
				app.RunChecks(ctx, cfg, initializedNotifiers, checker.Check)
				log.Printf("Checks complete. Will run again in %s.", checkInterval)
			}
		}
	}
}
