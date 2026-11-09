package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"tailwatch/internal/app"
	"tailwatch/internal/config"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil && !errors.Is(err, flag.ErrHelp) && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "tailwatch:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("tailwatch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "config.json", "Path to the JSON configuration")
	state := flags.String("state", "tailwatch-state.json", "Path to the live journal checkpoint")
	replay := flags.String("replay", "", "Read a journal JSON-lines file once and print events to stdout")
	verbose := flags.Bool("verbose", false, "Enable debug logging on stderr")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: level}))
	daemon := app.New(cfg, logger, stdout, *replay != "")
	if *replay == "" {
		return daemon.Follow(ctx, *state, stderr)
	}
	f, err := os.Open(*replay)
	if err != nil {
		return err
	}
	defer f.Close()
	return daemon.Replay(ctx, f)
}
