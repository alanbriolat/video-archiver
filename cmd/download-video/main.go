package main

import (
	"context"
	"flag"
	"log"
	"strings"

	"github.com/schollz/progressbar/v3"
	"go.uber.org/zap"

	"github.com/alanbriolat/video-archiver"
	"github.com/alanbriolat/video-archiver/generic"
	_ "github.com/alanbriolat/video-archiver/providers"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	// TODO: use the cancel on ctrl+c
	ctx, _ := context.WithCancel(video_archiver.WithLogger(context.Background(), logger))

	source := flag.String("source", "", "URL to the video page")
	target := flag.String("target", "", "Path to save the video at (must be a directory)")
	flag.Parse()

	if len(*source) == 0 || len(*target) == 0 {
		logger.Fatal("usage: download-video --source https://site.com/videos/id --target /path/to/downloads/")
	}

	logger.Sugar().Infof("Downloading from %s into %s", *source, *target)

	match, err := video_archiver.DefaultProviderRegistry.Match(*source)
	if err != nil {
		logger.Sugar().Fatalf("Match failed: %v", err)
	}

	logger.Info("Starting recon...")
	resolved, err := match.Source.Recon(ctx)
	if err != nil {
		logger.Sugar().Fatalf("Recon failed: %v", err)
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
	downloadBuilder.WithTargetPrefix(strings.TrimRight(*target, "/") + "/")
	download := generic.Unwrap(downloadBuilder.Build())
	defer download.Close()

	err = resolved.Download(download)
	if err != nil {
		logger.Sugar().Fatalf("Download failed: %v", err)
	}
	logger.Info("Download complete!")
}
