package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"

	"github.com/r3labs/diff/v3"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/alanbriolat/video-archiver/async"
	"github.com/alanbriolat/video-archiver/internal/session"
	"github.com/alanbriolat/video-archiver/internal/sync_"
	_ "github.com/alanbriolat/video-archiver/providers"
)

func main() {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logger, err := config.Build()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()
	zap.RedirectStdLog(logger)
	zap.ReplaceGlobals(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app := &cli.App{
		Name:  "download-video",
		Usage: "download a single video",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "target",
				Value: ".",
				Usage: "save downloaded video to `DIR`",
			},
		},
		Action: func(c *cli.Context) error {
			target := c.String("target")
			for _, source := range c.Args().Slice() {
				if err := download(ctx, source, target); err != nil {
					return err
				}
			}
			return nil
		},
		HideHelpCommand: true,
	}

	result := async.Run(func() error { return app.Run(os.Args) })

	select {
	case err = <-result:
		if err != nil {
			logger.Fatal(err.Error())
		}
	case <-ctx.Done():
		stop()
		err = <-result
		if err != nil {
			logger.Fatal(err.Error())
		}
	}
}

func download(ctx context.Context, source string, target string) error {
	logger := zap.S()
	logger.Infof("Downloading from %s into %s", source, target)

	cfg := session.DefaultConfig
	cfg.DefaultSavePath = target
	ses, err := session.New(cfg, ctx)
	if err != nil {
		return err
	}
	defer ses.Close()

	events, err := ses.Subscribe()
	if err != nil {
		return err
	}
	var started sync_.Event
	var stopped sync_.Event
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for event := range events.Receive() {
			logger.Debugf("event: %T: %v", event, event.Download())
			switch e := event.(type) {
			case session.DownloadStarted:
				started.Set()
			case session.DownloadStopped:
				stopped.Set()
			case session.DownloadUpdated:
				changes, err := diff.Diff(e.OldState, e.NewState)
				if err != nil {
					logger.Errorf("failed to diff old and new download state: %v", err)
				} else {
					for _, change := range changes {
						logger.Debugf("%v: %#v -> %#v", change.Path, change.From, change.To)
					}
				}
			}
		}
	}()

	dl, err := ses.AddDownload(source, nil)
	if err != nil {
		return err
	}
	logger.Info("Starting download")
	dl.Start()
	<-started.Wait()

	select {
	case <-stopped.Wait():
		if dl.IsComplete() {
			logger.Info("Download complete")
		} else {
			logger.Info("Download stopped")
		}
	case <-ctx.Done():
		logger.Info("Exiting gracefully...")
	}

	ses.Close()
	wg.Wait()

	return nil
}
