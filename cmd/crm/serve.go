package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/fdelillo/crm/db/migrations"
	"github.com/fdelillo/crm/internal/app"
	"github.com/fdelillo/crm/internal/identity"
	"github.com/fdelillo/crm/internal/identity/emails"
	"github.com/fdelillo/crm/internal/industrytemplate"
	"github.com/fdelillo/crm/internal/platform/audit"
	"github.com/fdelillo/crm/internal/platform/clock"
	"github.com/fdelillo/crm/internal/platform/config"
	"github.com/fdelillo/crm/internal/platform/db"
	"github.com/fdelillo/crm/internal/platform/mailer"
	"github.com/fdelillo/crm/internal/platform/objectstore"
	"github.com/fdelillo/crm/internal/platform/outbox"
	"github.com/fdelillo/crm/internal/platform/password"
	"github.com/fdelillo/crm/internal/tenant"
	"github.com/jackc/pgx/v5/pgxpool"
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
	expected, err := migrations.ExpectedVersion(migrations.FS)
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
	app.LogServerVersion(ctx, runner, logger)
	c := clock.Real{}
	hasher := password.NewHasher(4)
	recorder := audit.NewRecorder()
	users := identity.NewService(runner, outbox.NewEnqueuer(c), c, cfg.SessionIdle, cfg.SessionAbsolute,
		identity.WithAuthentication(hasher, recorder, cfg.AuthHMACKey, logger))
	storage, err := objectstore.NewS3(objectstore.S3Config{Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket,
		AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, UseSSL: cfg.S3UseSSL})
	if err != nil {
		return err
	}
	companies := tenant.NewService(runner, users, industrytemplate.NoopSeeder{}, hasher, recorder, logger, tenant.WithObjectStorage(storage))
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
	dispatcher := outbox.NewDispatcher(runner, emailHandler, c, logger, app.PeriodicTasks(runner, c, logger)...)

	api := app.BuildAPIRouter(users, companies, c, logger)
	root := app.NewRootHandler(app.RootDeps{
		API:       api,
		Liveness:  app.LivenessHandler(),
		Readiness: app.ReadinessHandler(runner, expected, logger),
		// Until the web package embeds the built SPA (T-F008) the interface answers 503.
		SPA: app.SPAUnavailableHandler(),
	}, app.NewCommonMiddleware(logger, cfg.IsLocal(), cfg.TrustedProxies))

	// A bad certificate pair fails here, before anything listens.
	srv, err := app.NewServer(cfg, root)
	if err != nil {
		return err
	}
	serverCtx, stopServers := context.WithCancel(ctx)
	defer stopServers()
	var lc net.ListenConfig
	ln, err := lc.Listen(serverCtx, "tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listening on HTTP_ADDR %s: %w", cfg.HTTPAddr, err)
	}
	// Serve owns each listener. Starting this goroutine here also releases HTTP_ADDR
	// if the second bind fails, without retaining or reusing the listener.
	apiDone := make(chan error, 1)
	go func() {
		apiDone <- app.Serve(serverCtx, srv, ln, logger)
		stopServers()
	}()
	metricsSrv := app.NewMetricsServer(cfg)
	metricsLn, err := lc.Listen(serverCtx, "tcp", cfg.MetricsAddr)
	if err != nil {
		stopServers()
		if serveErr := <-apiDone; serveErr != nil {
			return serveErr
		}
		return fmt.Errorf("listening on METRICS_ADDR %s: %w", cfg.MetricsAddr, err)
	}
	metricsDone := make(chan error, 1)
	go func() {
		metricsDone <- app.Serve(serverCtx, metricsSrv, metricsLn, logger.With("interface", "metrics"))
		stopServers()
	}()
	workerCtx, stopWorker := context.WithCancel(serverCtx)
	defer stopWorker()
	workerDone := make(chan error, 1)
	go func() { workerDone <- dispatcher.Run(workerCtx) }()
	serveErr := <-apiDone
	metricsErr := <-metricsDone
	if serveErr == nil {
		serveErr = metricsErr
	}
	stopWorker()
	select {
	case workerErr := <-workerDone:
		if serveErr == nil && workerErr != nil {
			return workerErr
		}
	case <-time.After(app.ShutdownTimeout):
		return fmt.Errorf("outbox worker did not stop within %s", app.ShutdownTimeout)
	}
	return serveErr
}
