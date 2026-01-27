package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/simulot/immich-go/app/root"
	"github.com/spf13/pflag"
)

// immich-go entry point
func main() {
	ctx := context.Background()
	if err := immichGoMain(ctx); err != nil {
		var sigErr signalError
		if errors.As(err, &sigErr) {
			os.Exit(signalExitCode(sigErr.signal))
		}
		os.Exit(1)
	}
}

type signalError struct {
	signal os.Signal
}

func (e signalError) Error() string {
	return fmt.Sprintf("received %s", e.signal)
}

func signalExitCode(sig os.Signal) int {
	if signal, ok := sig.(syscall.Signal); ok {
		return 128 + int(signal)
	}
	return 1
}

// makes immich-go breakable with signals and run it
func immichGoMain(ctx context.Context) error {
	// Create a context with cancel function to gracefully handle termination signals
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	c, a := root.RootImmichGoCommand(ctx)
	c.SilenceErrors = true
	c.SilenceUsage = true

	// Handle Ctrl+C and SIGTERM signals
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signalChannel)

	// Watch for signals to be received
	go func() {
		select {
		case sig := <-signalChannel:
			fmt.Fprintf(os.Stderr, "\nReceived %s. Shutting down...\n", sig)
			cancel(signalError{signal: sig})
		case <-ctx.Done():
		}
	}()
	// let's start
	executed, err := c.ExecuteContextC(ctx)
	if executed == nil {
		executed = c
	}
	if err == nil || errors.Is(err, pflag.ErrHelp) {
		return nil
	}

	cause := context.Cause(ctx)
	var sigErr signalError
	if errors.As(cause, &sigErr) {
		return sigErr
	}
	if cause != nil {
		err = cause
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	if a.Log().GetSLog() != nil {
		a.Log().Error(err.Error())
	}

	executed.PrintErrln(executed.ErrPrefix(), err.Error())
	_ = executed.Usage()
	return err
}
