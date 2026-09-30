// Command workbench starts the Inspector development harness web server.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/miroslav-matejovsky/inspector/harness/inspected"
	"github.com/miroslav-matejovsky/inspector/harness/workbench"
)

func main() {
	cfg, err := workbench.ParseConfig(os.Args[1:], os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Printf("workbench listening on http://%s (inspected at http://%s%s/)\n",
		cfg.Addr, cfg.Addr, inspected.PathPrefix)
	if err := workbench.Run(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
