// Command workbench starts the Inspector development harness web server.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
	"github.com/miroslav-matejovsky/inspector/harness/workbench"
	"github.com/miroslav-matejovsky/inspector/internal/logfile"
)

func main() { os.Exit(run()) }

// run returns the exit code: 2 for invalid flags, 1 for a failure, 0 after a
// clean shutdown. Deferred calls run before os.Exit because they belong to run.
func run() int {
	cfg, err := workbench.ParseConfig(os.Args[1:], os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	logFile, err := logfile.Open(cfg.LogDir, "workbench", cfg.LogLevel, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Printf("workbench listening on http://%s (inspected at http://%s%s/), log %s\n",
		cfg.Addr, cfg.Addr, inspected.PathPrefix, logFile.Path())
	code := 0
	if err := workbench.Run(ctx, cfg, logFile.Logger()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	if err := logFile.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	return code
}
