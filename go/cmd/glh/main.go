// Command glh is the Go harness launcher: it composes a profile, exposes the
// composed config, serves the web client, and runs one headless task.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"git.glkj.example.com/glkj/harness/go/internal/api"
	"git.glkj.example.com/glkj/harness/go/internal/boot"
	"git.glkj.example.com/glkj/harness/go/internal/config"
	"git.glkj.example.com/glkj/harness/go/internal/core/agent"
	"git.glkj.example.com/glkj/harness/go/internal/core/session"
	"git.glkj.example.com/glkj/harness/go/internal/llm"
	"git.glkj.example.com/glkj/harness/go/internal/plugin"
)

// Version is stamped at build time.
var Version = "0.0.0-g0"

func main() {
	if err := rootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "glh: %v\n", err)
		os.Exit(1)
	}
}

func rootCommand() *cobra.Command {
	var (
		profile       string
		patches       []string
		port          int
		repoDir       string
		installAnchor string
		logLevel      string
	)

	root := &cobra.Command{
		Use:           "glh",
		Short:         "Boot a GLKJ Harness profile",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&profile, "profile", "", "profile name (default web)")
	root.PersistentFlags().StringSliceVar(&patches, "patch", nil, "extra patch overlay, in argv order")
	root.PersistentFlags().StringVar(&repoDir, "repo", "", "repository root used to locate apps/web/dist and the install anchor")
	root.PersistentFlags().StringVar(&installAnchor, "install-anchor", "", "directory whose node_modules holds the bundle packages")
	root.PersistentFlags().StringVar(&logLevel, "log-level", "info", "debug, info, warn, error")

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the harness version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), Version)
			return nil
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "dump-config",
		Short: "Print the composed plugin tree without starting it",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := loadConfig(profile, repoDir, installAnchor)
			dump, err := composeDump(cfg, patches, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), dump)
			return nil
		},
	})

	webCommand := &cobra.Command{
		Use:   "web",
		Short: "Serve the web client",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := loadConfig(profile, repoDir, installAnchor)
			compose := func() (string, error) { return composeDump(cfg, patches, io.Discard) }
			if _, err := compose(); err != nil {
				return err
			}
			server := &api.Server{Config: cfg, Compose: compose, RepoDir: repoDir}
			listener := api.Addr("127.0.0.1", port)
			httpServer := &http.Server{Addr: listener, Handler: server.Handler(), ReadHeaderTimeout: 10 * time.Second}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = httpServer.Shutdown(shutdownCtx)
			}()

			fmt.Fprintf(cmd.OutOrStdout(), "glh web: http://%s\n", listener)
			if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("serve: %w", err)
			}
			return nil
		},
	}
	webCommand.Flags().IntVar(&port, "port", 3080, "listen port; 0 picks a free one")
	root.AddCommand(webCommand)

	var jsonOutput bool
	headless := &cobra.Command{
		Use:   "headless [task]",
		Short: "Run one task without a server",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadConfig(profile, repoDir, installAnchor)
			logger := newLogger(logLevel)
			ctx := plugin.New(logger)
			defer ctx.Dispose()

			log := session.NewLog(fmt.Sprintf("headless-%d", time.Now().UnixMilli()))
			model := cfg.Model
			if model == "" {
				model = "deepseek-chat"
			}
			runner := agent.New(ctx, log, agent.Options{Provider: llm.NewDeepSeek(cfg.BaseURL, cfg.APIKey), Model: model})
			if err := runner.Run(cmd.Context(), args[0]); err != nil {
				return err
			}
			if jsonOutput {
				encoded, err := json.MarshalIndent(log.Events(), "", "  ")
				if err != nil {
					return fmt.Errorf("encode session: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return nil
			}
			messages := log.DeriveMessages()
			if len(messages) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), messages[len(messages)-1].Text)
			}
			return nil
		},
	}
	headless.Flags().BoolVar(&jsonOutput, "json", false, "print the full session log as JSON")
	root.AddCommand(headless)

	return root
}

func loadConfig(profile, repoDir, anchor string) config.Config {
	cfg := config.Load(profile)
	cfg.RepoDir = repoDir
	cfg.Anchor = anchor
	return cfg
}

func newLogger(level string) *slog.Logger {
	var parsed slog.Level
	if err := parsed.UnmarshalText([]byte(level)); err != nil {
		parsed = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: parsed}))
}

// composeDump stacks the profile's bundles, the profile patch, the home patch,
// and any --patch overlays over an empty entry list.
func composeDump(cfg config.Config, overlays []string, warn io.Writer) (string, error) {
	layers, err := boot.StackLayers(cfg.ProfilePath(), cfg.Home, cfg.InstallAnchor(), overlays)
	if err != nil {
		return "", err
	}
	tree := boot.NewTree()
	for _, layer := range layers {
		if err := tree.Apply(layer); err != nil {
			return "", err
		}
	}
	for _, warning := range tree.Warnings() {
		fmt.Fprintf(warn, "glh: %s\n", warning)
	}
	return tree.Dump()
}
