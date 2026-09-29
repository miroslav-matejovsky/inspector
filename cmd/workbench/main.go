// Command workbench starts the Inspector development harness web server.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/miroslav-matejovsky/inspector/harness/workbench"
)

func main() {
	addr := flag.String("addr", "", "listen address, for example localhost:8080 (required)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Printf("workbench listening on http://%s\n", *addr)
	if err := workbench.Run(ctx, *addr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
