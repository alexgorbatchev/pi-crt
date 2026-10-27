package media

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func source(ctx context.Context, manifestPath, src string, log io.Writer) (string, error) {
	u, err := url.Parse(src)
	if err != nil {
		return "", err
	}
	if u.Scheme == "https" || u.Scheme == "http" {
		return downloadSource(ctx, filepath.Join(filepath.Dir(manifestPath), "sources"), u, log)
	}
	if u.Scheme != "" {
		return "", fmt.Errorf("unsupported source scheme %q", u.Scheme)
	}
	if !filepath.IsAbs(src) {
		src = filepath.Join(filepath.Dir(manifestPath), src)
	}
	return filepath.Abs(src)
}

// Keep original streams and provenance independently of any prepared library.
// Publish only a complete, validated download; an interrupted render cannot
// delete the source or leave a partial download eligible for reuse.
func downloadSource(ctx context.Context, root string, u *url.URL, log io.Writer) (path string, err error) {
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(root, fmt.Sprintf("%x", sha256.Sum256([]byte(u.String()))))
	host := strings.ToLower(u.Hostname())
	youtube := host == "youtu.be" || host == "youtube.com" || strings.HasSuffix(host, ".youtube.com")
	path = filepath.Join(dest, "source.mkv")
	if _, err := os.Stat(dest); err == nil {
		return path, validateDownload(ctx, path, youtube, log)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(root, ".download-")
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(stage)) }()
	format := "bv*[height<=?720]+ba/b[height<=?720]"
	if youtube {
		format = "bv*[height=720]+ba/b[height=720]"
	}
	if err := run(ctx, log, "yt-dlp", "--ignore-config", "--no-playlist", "--no-progress", "--no-part", "--no-cache-dir", "--write-info-json", "--merge-output-format", "mkv", "--remux-video", "mkv", "-S", "vcodec:h264,acodec:aac", "-f", format, "-o", filepath.Join(stage, "source.%(ext)s"), "--", u.String()); err != nil {
		return "", err
	}
	if err := validateDownload(ctx, filepath.Join(stage, "source.mkv"), youtube, log); err != nil {
		return "", err
	}
	if err := os.Rename(stage, dest); err != nil {
		return "", err
	}
	return path, nil
}

func validateDownload(ctx context.Context, path string, youtube bool, log io.Writer) error {
	info, err := inspect(ctx, path)
	if err != nil {
		return fmt.Errorf("retained source %s: %w", path, err)
	}
	if youtube && (info.Height != 720 || !info.Audio) {
		return fmt.Errorf("YouTube source %s requires genuine 720-line video and audio", path)
	}
	return run(ctx, log, "ffmpeg", "-v", "error", "-xerror", "-nostdin", "-i", path, "-map", "0:v:0", "-map", "0:a?", "-f", "null", "-")
}
