package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/alanbriolat/video-archiver/generic"
	"github.com/alanbriolat/video-archiver/gui"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	// Ensure the signal handler is cleaned up so repeated signals are less graceful
	go func() {
		<-ctx.Done()
		stop()
	}()

	builder := gui.NewApplicationBuilder(gui.DefaultAppName, gui.DefaultAppID).WithContext(ctx)
	app := generic.Unwrap(builder.Build())
	generic.Unwrap_(app.Init())
	defer app.Close()

	exitCode := app.Run()
	os.Exit(exitCode)
}
