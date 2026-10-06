package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/identity/emails"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/config"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/mailer"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/platform/ratelimit"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
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
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("DATABASE_URL: %w", err)
	}
	defer pool.Close()
	runner := db.NewTxRunner(pool, db.WithLogger(logger))
	c := clock.Real{}
	hasher := password.NewHasher(4)
	recorder := audit.NewRecorder()
	users := identity.NewService(runner, outbox.NewEnqueuer(c), c, cfg.SessionIdle, cfg.SessionAbsolute,
		identity.WithAuthentication(hasher, recorder, cfg.AuthHMACKey, logger))
	companies := tenant.NewService(runner, users, industrytemplate.NoopSeeder{}, hasher, recorder, logger)
	limiter := ratelimit.NewLimiter(rate.Every(12*time.Minute), 5, c, time.Hour)
	smtp, err := mailer.NewSMTP(mailer.SMTPConfig{Host: cfg.SMTPHost, Port: cfg.SMTPPort,
		Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, From: cfg.SMTPFrom})
	if err != nil {
		return err
	}
	emailHandler, err := emails.NewHandler(smtp, cfg.AppBaseURL.String(),
		emails.LinkPaths{Reset: cfg.AppLinkReset, Verify: cfg.AppLinkVerify, Invitation: cfg.AppLinkInvitation})
	if err != nil {
		return err
	}
	dispatcher := outbox.NewDispatcher(runner, emailHandler, c, logger)

	api := app.NewAPIRouter()
	industrytemplate.RegisterRoutes(api)
	tenant.RegisterRoutes(api, companies, limiter, logger)
	app.RegisterAuthRoutes(api, users, companies, ratelimit.NewLimiter(rate.Every(3*time.Second), 20, c, time.Minute), logger)
	app.RegisterMeRoute(api, users, companies, logger)
	app.RegisterRecoveryRoutes(api, users, ratelimit.NewLimiter(rate.Every(12*time.Minute), 5, c, time.Hour),
		ratelimit.NewLimiter(rate.Every(3*time.Minute), 20, c, time.Hour), logger)
	app.RegisterUserRoutes(api, users, companies, ratelimit.NewLimiter(rate.Every(3*time.Minute), 20, c, time.Hour), logger)
	root := app.NewRootHandler(app.RootDeps{
		API:       api,
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
	workerCtx, stopWorker := context.WithCancel(ctx)
	defer stopWorker()
	workerDone := make(chan error, 1)
	go func() { workerDone <- dispatcher.Run(workerCtx) }()
	serveErr := app.Serve(ctx, srv, ln, logger)
	stopWorker()
	select {
	case workerErr := <-workerDone:
		if serveErr == nil && workerErr != nil {
			return workerErr
		}
	case <-time.After(25 * time.Second):
		return fmt.Errorf("outbox worker did not stop within 25 seconds")
	}
	return serveErr
}
