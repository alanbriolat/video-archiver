package util

import (
	"errors"
	"net/url"
	"strings"
)

var (
	ErrNoFilename = errors.New("cannot extract valid filename")
)

func FilenameFromURL(url *url.URL) (string, error) {
	if url == nil {
		return "", ErrNoFilename
	}
	path := strings.Trim(url.Path, "/")
	if path == "" {
		return "", ErrNoFilename
	}
	pathElements := strings.Split(path, "/")
	filename := pathElements[len(pathElements)-1]
	if filename == "" {
		return "", ErrNoFilename
	}
	// Don't allow "filenames" that are just ".", "..", etc.
	if strings.ReplaceAll(filename, ".", "") == "" {
		return "", ErrNoFilename
	}
	return filename, nil
}

func FilenameFromURLString(s string) (string, error) {
	if parsedURL, err := url.Parse(s); err != nil {
		return "", err
	} else {
		return FilenameFromURL(parsedURL)
	}
}
