package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver/async"
	"github.com/alanbriolat/video-archiver/internal/session"
	_ "github.com/alanbriolat/video-archiver/providers"
)

func main() {
	logger, err := zap.NewDevelopment()
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
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for event := range events.Receive() {
			logger.Debugf("event: %T: %v", event, event)
		}
	}()

	dl, err := ses.AddDownload(source, nil)
	if err != nil {
		return err
	}
	dl.Start()
	<-dl.Running()
	//dl.Stop()
	//<-dl.Stopped()
	//dl.Close()
	//<-dl.Done()

	select {
	case <-dl.Complete():
		logger.Info("Download complete")
	case <-ctx.Done():
		logger.Info("Exiting gracefully...")
	}

	ses.Close()
	wg.Wait()

	return nil
}
