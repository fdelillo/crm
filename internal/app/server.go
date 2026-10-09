package app

import (
	"context"
	"crypto/tls"
	"errors"
	"expvar"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/fdelillo/crm/internal/platform/config"
	"github.com/fdelillo/crm/internal/platform/outbox"
)

// Server timeouts (plan §10.3). IdleTimeout is not in the plan; 60 s is the usual keep-alive window.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second

	// ShutdownTimeout bounds how long Serve waits for in-flight requests after the context is cancelled.
	ShutdownTimeout = outbox.SendBudget + 5*time.Second
)

// NewServer builds the *http.Server with the timeouts of plan §10.3. With cfg.TLS set (local mode
// only, DD-24) it loads the certificate pair now, before anything listens; a bad pair is an error
// that names TLS_CERT_FILE/TLS_KEY_FILE and never includes file contents.
func NewServer(cfg config.Config, root http.Handler) (*http.Server, error) {
	if root == nil {
		return nil, errors.New("app: root handler is nil")
	}
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           root,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	if cfg.TLS != nil {
		cert, err := loadKeyPair(*cfg.TLS)
		if err != nil {
			return nil, err
		}
		srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	}
	return srv, nil
}

// NewMetricsServer exposes only GET /debug/vars on a private mux, without TLS or API middleware.
func NewMetricsServer(cfg config.Config) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /debug/vars", expvar.Handler())
	return &http.Server{Addr: cfg.MetricsAddr, Handler: mux,
		ReadHeaderTimeout: readHeaderTimeout, ReadTimeout: readTimeout,
		WriteTimeout: writeTimeout, IdleTimeout: idleTimeout}
}

// loadKeyPair reads each file separately so the error can name the variable at fault.
func loadKeyPair(f config.TLSFiles) (tls.Certificate, error) {
	certPEM, err := os.ReadFile(f.CertFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("TLS_CERT_FILE: %w", err)
	}
	keyPEM, err := os.ReadFile(f.KeyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("TLS_KEY_FILE: %w", err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		// The parse error does not carry file contents, but it cannot say which file is at fault.
		return tls.Certificate{}, fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE are not a valid certificate/key pair: %w", err)
	}
	return cert, nil
}

// Serve runs srv on ln until ctx is cancelled (SIGTERM in production), then shuts down gracefully:
// in-flight requests finish, bounded by ShutdownTimeout. It logs the listen mode: https_local when
// the server has a certificate (DD-24), http otherwise. It returns nil after a clean shutdown.
func Serve(ctx context.Context, srv *http.Server, ln net.Listener, logger *slog.Logger) error {
	mode := "http"
	if srv.TLSConfig != nil {
		mode = "https_local"
	}
	if srv.ErrorLog == nil {
		// net/http's own messages (TLS handshake failures, ...) would otherwise go to stderr as plain text.
		srv.ErrorLog = slog.NewLogLogger(logger.Handler(), slog.LevelWarn)
	}
	logger.InfoContext(ctx, "server listening", "listen", mode, "addr", ln.Addr().String())

	errc := make(chan error, 1)
	go func() {
		if srv.TLSConfig != nil {
			errc <- srv.ServeTLS(ln, "", "") // certificates come from TLSConfig
			return
		}
		errc <- srv.Serve(ln)
	}()

	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serving HTTP: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	// The parent context is already cancelled: the shutdown deadline must not derive from it.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
		return fmt.Errorf("shutting down HTTP server: %w", err)
	}
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving HTTP: %w", err)
	}
	return nil
}
