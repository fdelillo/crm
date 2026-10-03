package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/platform/config"
)

// runServe starts the HTTP server: HTTP by default, HTTPS with the local certificate when
// TLS_CERT_FILE and TLS_KEY_FILE are set in local mode (DD-24). It returns nil after a graceful
// shutdown (ctx cancelled by SIGTERM).
func runServe(ctx context.Context, args []string, e env) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	if err := fs.Parse(args); err != nil {
		return usagef("serve: %v", err)
	}
	if fs.NArg() > 0 {
		return usagef("serve: unexpected argument %q", fs.Arg(0))
	}

	cfg, err := config.Load(e.getenv)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(e.stdout, nil))
	// Behind a proxy that is not listed here every client looks like the proxy (one shared rate limit
	// bucket, useless IPs in logs and audit): the first line says what is trusted (DD-32).
	trusted := make([]string, 0, len(cfg.TrustedProxies))
	for _, p := range cfg.TrustedProxies {
		trusted = append(trusted, p.String())
	}
	logger.Info("starting", "local_mode", cfg.IsLocal(), "trusted_proxies", trusted)

	root := app.NewRootHandler(app.RootDeps{
		API:       app.NewAPIRouter(),
		Liveness:  app.LivenessHandler(),
		Readiness: app.ReadinessPlaceholder(),
		// Until the web package embeds the built SPA (T-F008) the interface answers 503.
		SPA: app.SPAUnavailableHandler(),
	}, app.NewCommonMiddleware(logger, cfg.IsLocal(), cfg.TrustedProxies))

	// A bad certificate pair fails here, before anything listens.
	srv, err := app.NewServer(cfg, root)
	if err != nil {
		return err
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listening on HTTP_ADDR %s: %w", cfg.HTTPAddr, err)
	}
	return app.Serve(ctx, srv, ln, logger)
}
