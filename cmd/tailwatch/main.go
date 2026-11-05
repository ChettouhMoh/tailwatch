package main

import (
    "context"
    "flag"
    "fmt"
    "log/slog"
    "os"
)

import "tailwatch/internal/app"

func main() {
    // Parse command line flags
    verbose := flag.Bool("verbose", false, "Enable verbose logging")
    configPath := flag.String("config", "config.json", "Path to configuration file")
    flag.Parse()

    // Set up logging
    var level slog.Level
    if *verbose {
        level = slog.LevelDebug
    } else {
        level = slog.LevelInfo
    }
    
    logger := slog.New(slog.NewJSONHandler(nil))
    
    // Load configuration
    // For now, use a simple config - in production, load from configPath
    config := struct {
    }{
        // TODO: Load actual config from file
    }
    
    // Create application
    application, err := app.NewApp(config, logger)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to create application: %v\n", err)
        os.Exit(1)
    }
    
    // Start application
    ctx := context.Background()
    if err := application.Start(ctx); err != nil {
        logger.Error("Application error", "error", err)
        os.Exit(1)
    }
    
    logger.Info("Application completed successfully")
}
