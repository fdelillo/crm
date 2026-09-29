// Command crm is the single binary of the CRM: the HTTP server (`serve`), the database
// migrations (`migrate`) and operational commands (`tenants`).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata" // embeds the IANA zone database so zones resolve in minimal images (DD-27)
)

const usageText = `Usage: crm <command> [arguments]

Commands:
  serve                        start the HTTP server
  migrate up|down|status       run the embedded database migrations (uses DATABASE_MIGRATION_URL)
  tenants reprovision-roles    recreate the PostgreSQL role of every company (operations)

Configuration comes from environment variables; see .env.example.
`

// usageError is a mistake in how the command was invoked; main exits with status 2 for it.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func usagef(format string, args ...any) error { return &usageError{msg: fmt.Sprintf(format, args...)} }

func main() {
	// SIGTERM is what the hosting sends to stop the process: serve shuts down gracefully on it.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "crm:", err)
	var ue *usageError
	if errors.As(err, &ue) {
		fmt.Fprint(os.Stderr, "\n", usageText)
		os.Exit(2)
	}
	os.Exit(1)
}

// env is what every command needs from the outside world; tests replace it.
type env struct {
	getenv func(string) string
	stdout io.Writer
	stderr io.Writer
}

// run dispatches the subcommand. It is separate from main so tests can call it.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usagef("missing command")
	}
	e := env{getenv: getenv, stdout: stdout, stderr: stderr}
	switch args[0] {
	case "serve":
		return runServe(ctx, args[1:], e)
	case "migrate":
		return runMigrate(ctx, args[1:], e)
	case "tenants":
		return runTenants(ctx, args[1:], e)
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usageText)
		return nil
	default:
		return usagef("unknown command %q", args[0])
	}
}
