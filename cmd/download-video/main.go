package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/async"
	"github.com/alanbriolat/video-archiver/generic"
	_ "github.com/alanbriolat/video-archiver/providers"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()
	zap.RedirectStdLog(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx = video_archiver.WithLogger(ctx, logger)

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
		logger.Error(ctx.Err().Error())
		stop()
	}
}

func download(ctx context.Context, source string, target string) error {
	logger := video_archiver.Logger(ctx).Sugar()
	logger.Infof("Downloading from %s into %s", source, target)

	match, err := video_archiver.DefaultProviderRegistry.Match(source)
	if err != nil {
		return fmt.Errorf("match failed: %w", err)
	}

	logger.Info("Starting recon...")
	resolved, err := match.Source.Recon(ctx)
	if err != nil {
		return fmt.Errorf("recon failed: %w", err)
	}

	logger.Info("Starting download...")
	bar := progressbar.DefaultBytes(1, "downloading")
	downloadBuilder := video_archiver.NewDownloadBuilder()
	downloadBuilder.WithContext(ctx)
	downloadBuilder.WithProgressCallback(func(downloaded int, expected int) {
		if bar.GetMax() != expected {
			bar.ChangeMax(expected)
		}
		generic.Unwrap_(bar.Set(downloaded))
	})
	downloadBuilder.WithTargetPrefix(strings.TrimRight(target, "/") + "/")
	download := generic.Unwrap(downloadBuilder.Build())
	defer download.Close()

	err = resolved.Download(download)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	logger.Info("Download complete!")

	return nil
}
