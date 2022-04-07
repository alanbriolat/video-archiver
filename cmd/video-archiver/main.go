package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver/generic"
	"github.com/alanbriolat/video-archiver/gui"
	_ "github.com/alanbriolat/video-archiver/providers"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize logger: %v", err)
	}
	defer logger.Sync()
	zap.RedirectStdLog(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	// Ensure the signal handler is cleaned up so repeated signals are less graceful
	go func() {
		<-ctx.Done()
		stop()
	}()

	env := generic.Unwrap(gui.NewEnvBuilder().Context(ctx).Logger(logger).UserConfigDir(gui.DefaultAppName).Build())
	app := generic.Unwrap(gui.NewApplication(env, gui.DefaultAppID))

	exitCode := app.Run()
	os.Exit(exitCode)
}
