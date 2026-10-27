// Package media prepares low-resolution H.264 clips on the editing computer.
package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"pi-crt/internal/library"
)

type Cut struct {
	ID     string  `json:"id"`
	Group  string  `json:"group"`
	Source string  `json:"source"`
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Crop   *Crop   `json:"crop,omitempty"`
}

// Crop identifies the active picture in source pixels, before fitting to 4:3.
type Crop struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}
type Manifest struct {
	Clips []Cut `json:"clips"`
}
type Options struct {
	Standard string
	Noise    int
	Log      io.Writer
}

type probe struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
}

type mediaInfo struct {
	Duration      float64
	Audio         bool
	Width, Height int
}

func run(ctx context.Context, log io.Writer, name string, args ...string) error {
	c := exec.CommandContext(ctx, name, args...)
	c.Stdout, c.Stderr = log, log
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func inspect(ctx context.Context, path string) (mediaInfo, error) {
	b, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_format", "-show_streams", "-of", "json", path).Output()
	if err != nil {
		return mediaInfo{}, fmt.Errorf("probe %s: %w", path, err)
	}
	var p probe
	if err := json.Unmarshal(b, &p); err != nil {
		return mediaInfo{}, err
	}
	d, err := strconv.ParseFloat(p.Format.Duration, 64)
	if err != nil || !library.ValidDuration(d) {
		return mediaInfo{}, fmt.Errorf("%s: missing finite duration", path)
	}
	info := mediaInfo{Duration: d}
	for _, s := range p.Streams {
		if s.CodecType == "video" && info.Width == 0 {
			info.Width, info.Height = s.Width, s.Height
		}
		info.Audio = info.Audio || s.CodecType == "audio"
	}
	if info.Width <= 0 || info.Height <= 0 {
		return mediaInfo{}, fmt.Errorf("%s: no video stream with valid dimensions", path)
	}
	return info, nil
}

func Prepare(ctx context.Context, manifestPath, output string, opts Options) (err error) {
	if opts.Standard != "ntsc" && opts.Standard != "pal" {
		return fmt.Errorf("standard must be ntsc or pal")
	}
	if opts.Noise < 0 || opts.Noise > 30 {
		return fmt.Errorf("noise must be between 0 and 30")
	}
	m, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(output); err == nil {
		return fmt.Errorf("output %s already exists; choose a new directory", output)
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			return err
		}
	}
	stage, err := os.MkdirTemp(filepath.Dir(output), ".pi-crt-prepare-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(stage)) }()
	if err := os.Mkdir(filepath.Join(stage, "clips"), 0755); err != nil {
		return err
	}
	lib := library.Library{Standard: opts.Standard}
	sources := make(map[string]string)
	for _, c := range m.Clips {
		if err := ctx.Err(); err != nil {
			return err
		}
		path, ok := sources[c.Source]
		if !ok {
			path, err = source(ctx, manifestPath, stage, c.Source, len(sources), opts.Log)
			if err != nil {
				return err
			}
			sources[c.Source] = path
		}
		info, err := inspect(ctx, path)
		if err != nil {
			return err
		}
		if c.End > info.Duration+0.05 {
			return fmt.Errorf("cut %s ends at %.3fs beyond source duration %.3fs", c.ID, c.End, info.Duration)
		}
		if r := c.Crop; r != nil && (r.Width > info.Width || r.Height > info.Height || r.X > info.Width-r.Width || r.Y > info.Height-r.Height) {
			return fmt.Errorf("cut %s: crop exceeds source dimensions %dx%d", c.ID, info.Width, info.Height)
		}
		dest := filepath.Join("clips", c.ID+".mp4")
		if opts.Log != nil {
			if _, err := fmt.Fprintf(opts.Log, "Preparing %s\n", c.ID); err != nil {
				return err
			}
		}
		if err := render(ctx, path, filepath.Join(stage, dest), c, info.Audio, opts); err != nil {
			return fmt.Errorf("cut %s: %w", c.ID, err)
		}
		prepared, err := inspect(ctx, filepath.Join(stage, dest))
		if err != nil {
			return err
		}
		group := c.Group
		if group == "" {
			group = c.Source
		}
		lib.Clips = append(lib.Clips, library.Clip{ID: c.ID, Group: group, File: filepath.ToSlash(dest), Duration: prepared.Duration})
	}
	// Source downloads are preparation scratch, never part of the Pi library.
	for _, path := range sources {
		if filepath.Dir(path) == stage {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	b, err := json.MarshalIndent(lib, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "library.json"), append(b, '\n'), 0644); err != nil {
		return err
	}
	if err := os.Chmod(stage, 0755); err != nil {
		return err
	}
	return os.Rename(stage, output)
}

func source(ctx context.Context, manifestPath, stage, src string, index int, log io.Writer) (string, error) {
	u, err := url.Parse(src)
	if err != nil {
		return "", err
	}
	if u.Scheme == "https" || u.Scheme == "http" {
		path := filepath.Join(stage, fmt.Sprintf("source-%d.mkv", index))
		err := run(ctx, log, "yt-dlp", "--ignore-config", "--no-playlist", "--no-progress", "--no-part", "--no-cache-dir", "--merge-output-format", "mkv", "--remux-video", "mkv", "-f", "bv*[height<=?720]+ba/b[height<=?720]", "-o", path, "--", src)
		return path, err
	}
	if u.Scheme != "" {
		return "", fmt.Errorf("unsupported source scheme %q", u.Scheme)
	}
	if !filepath.IsAbs(src) {
		src = filepath.Join(filepath.Dir(manifestPath), src)
	}
	return filepath.Abs(src)
}

func render(ctx context.Context, src, dest string, cut Cut, audio bool, opts Options) error {
	d := cut.End - cut.Start
	rate := "30000/1001"
	if opts.Standard == "pal" {
		rate = "25"
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-ss", number(cut.Start), "-t", number(d), "-i", src}
	// A finite silence source also covers silent source videos and short audio tracks.
	args = append(args, "-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono")
	crop := ""
	if r := cut.Crop; r != nil {
		crop = fmt.Sprintf("crop=%d:%d:%d:%d:exact=1,", r.Width, r.Height, r.X, r.Y)
	}
	video := fmt.Sprintf("[0:v:0]setpts=PTS-STARTPTS,%sscale=640:480:force_original_aspect_ratio=increase:force_divisible_by=2:reset_sar=1,crop=640:480,setsar=1,fps=%s,format=yuv420p,noise=c0s=%d:c0f=t+u[v];", crop, rate, opts.Noise)
	a := "[1:a]"
	if audio {
		a = "[0:a:0]"
	}
	audioFilter := fmt.Sprintf("%sasetpts=PTS-STARTPTS,aresample=48000,aformat=channel_layouts=mono,apad,atrim=duration=%s,dynaudnorm=p=0.7:m=5:s=5,afade=t=in:d=0.02,afade=t=out:st=%s:d=0.03[a];", a, number(d), number(d-0.03))
	static := fmt.Sprintf("color=c=gray:s=640x480:r=%s:d=0.4,format=yuv420p,noise=c0s=100:c0f=t+u,fade=t=out:st=0.35:d=0.05[sv];anoisesrc=r=48000:d=0.4:a=0.06,afade=t=out:st=0.3:d=0.1,aformat=channel_layouts=mono[sa];", rate)
	graph := video + audioFilter + static + "[sv][sa][v][a]concat=n=2:v=1:a=1[outv][outa]"
	args = append(args, "-filter_complex", graph, "-map", "[outv]", "-map", "[outa]", "-c:v", "libx264", "-profile:v", "baseline", "-level:v", "3.0", "-preset", "fast", "-crf", "23", "-maxrate", "2M", "-bufsize", "4M", "-pix_fmt", "yuv420p", "-r", rate, "-g", "30", "-c:a", "aac", "-b:a", "96k", "-ar", "48000", "-ac", "1", "-movflags", "+faststart", "-map_metadata", "-1", dest)
	return run(ctx, opts.Log, "ffmpeg", args...)
}

func number(n float64) string { return strconv.FormatFloat(n, 'f', 6, 64) }
