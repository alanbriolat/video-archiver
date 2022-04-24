package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"

	"github.com/r3labs/diff/v3"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/alanbriolat/video-archiver/async"
	"github.com/alanbriolat/video-archiver/generic"
	"github.com/alanbriolat/video-archiver/internal/session"
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
			err := download(ctx, c.Args().Slice(), target)
			return err
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

func download(ctx context.Context, sources []string, target string) error {
	logger := zap.S()
	logger.Infof("Downloading into %s from %s", target, sources)

	cfg := session.DefaultConfig
	cfg.DefaultSavePath = target
	ses, err := session.New(cfg, ctx)
	if err != nil {
		return err
	}
	defer ses.Close()

	var downloads sync.WaitGroup
	for _, source := range sources {
		dl, err := ses.AddDownload(source, nil)
		if err != nil {
			logger.Errorf("failed to add download for %#v: %v", source, err)
			continue
		}
		downloads.Add(1)
		go func() {
			defer downloads.Done()
			initialState := generic.Unwrap(dl.State())
			logger := logger.Named(fmt.Sprintf("client/%v", initialState.ID))
			events := generic.Unwrap(dl.Subscribe())
			dl.Start()
			for event := range events.Receive() {
				logger.Debugf("event %T: %v", event, event.Download())
				switch e := event.(type) {
				case session.DownloadUpdated:
					changes, err := diff.Diff(e.OldState, e.NewState)
					if err != nil {
						logger.Errorf("failed to diff old and new download state: %v", err)
					} else {
						for _, change := range changes {
							logger.Debugf("%v: %#v -> %#v", change.Path, change.From, change.To)
						}
					}
				case session.DownloadStopped:
					state := generic.Unwrap(e.Download().State())
					logger.Infof("Download stopped with status %#v", state.Status)
					return
				}
			}
		}()
	}

	downloads.Wait()
	ses.Close()
	return nil
}
