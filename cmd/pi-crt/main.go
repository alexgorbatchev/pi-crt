package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"pi-crt/internal/agent"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := newRootCommand().ExecuteContext(ctx); err != nil && !errors.Is(err, context.Canceled) {
		if agent.IsAgentMode() {
			fmt.Fprintf(os.Stderr, "ERR: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		}
		os.Exit(1)
	}
}
