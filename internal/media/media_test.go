package media

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"pi-crt/internal/library"
)

func writeManifest(t *testing.T, dir string, cuts []Cut) string {
	t.Helper()
	path := filepath.Join(dir, "manifest.json")
	b, err := json.Marshal(Manifest{Clips: cuts})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPrepareValidation(t *testing.T) {
	dir := t.TempDir()
	good := Cut{ID: "a", Source: "x.mp4", Start: 0, End: 1}
	for _, cuts := range [][]Cut{nil, {{ID: "../a", Source: "x", End: 1}}, {{ID: "a", End: 1}}, {{ID: "a", Source: "x", End: 0.1}}, {good, good}} {
		if err := Prepare(t.Context(), writeManifest(t, dir, cuts), filepath.Join(dir, "out"), Options{Standard: "ntsc"}); err == nil {
			t.Fatal("accepted invalid manifest")
		}
	}
	p := writeManifest(t, dir, []Cut{good})
	for _, o := range []Options{{Standard: "bad"}, {Standard: "ntsc", Noise: -1}, {Standard: "ntsc", Noise: 31}} {
		if err := Prepare(t.Context(), p, dir, o); err == nil {
			t.Fatal("accepted options")
		}
	}
	if err := Prepare(t.Context(), p, dir, Options{Standard: "ntsc"}); err == nil {
		t.Fatal("overwrote directory")
	}
	if err := Prepare(t.Context(), "missing", dir, Options{Standard: "ntsc"}); err == nil {
		t.Fatal("accepted missing manifest")
	}
	if _, err := source(t.Context(), p, "ftp://example.com/a", io.Discard); err == nil {
		t.Fatal("accepted ftp")
	}
	if _, err := source(t.Context(), p, "http://%", io.Discard); err == nil {
		t.Fatal("accepted invalid URL")
	}
	t.Setenv("PATH", t.TempDir())
	if err := Prepare(t.Context(), p, filepath.Join(dir, "out"), Options{Standard: "ntsc"}); err == nil {
		t.Fatal("accepted missing ffmpeg")
	}
	if _, err := source(t.Context(), p, "https://example.com/video", io.Discard); err == nil {
		t.Fatal("accepted missing downloader")
	}
	entries, err := os.ReadDir(filepath.Join(dir, "sources"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed download left reusable source or scratch: %v %v", entries, err)
	}
}

func TestRenderIntegration(t *testing.T) {
	if os.Getenv("PI_CRT_INTEGRATION") != "1" {
		t.Skip("set PI_CRT_INTEGRATION=1; requires FFmpeg and FFprobe")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mp4")
	if err := run(t.Context(), os.Stderr, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=30:duration=3", "-f", "lavfi", "-i", "sine=frequency=440:duration=3", "-c:v", "libx264", "-c:a", "aac", src); err != nil {
		t.Fatal(err)
	}
	for _, standard := range []string{"ntsc", "pal"} {
		t.Run(standard, func(t *testing.T) {
			p := writeManifest(t, dir, []Cut{{ID: "one", Source: "source.mp4", Start: 0.5, End: 1.5}, {ID: "two", Group: "movie", Source: src, Start: 1, End: 2}})
			out := filepath.Join(dir, standard)
			if err := Prepare(t.Context(), p, out, Options{Standard: standard, Noise: 4, Log: os.Stderr}); err != nil {
				t.Fatal(err)
			}
			l, err := library.Load(out)
			if err != nil {
				t.Fatal(err)
			}
			if len(l.Clips) != 2 {
				t.Fatal("missing clip")
			}
			for _, c := range l.Clips {
				if c.Duration < 1.35 || c.Duration > 1.5 {
					t.Fatalf("bad cut duration %f", c.Duration)
				}
				if err := run(t.Context(), os.Stderr, "ffmpeg", "-v", "error", "-i", filepath.Join(out, c.File), "-f", "null", "-"); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	// Verify a silent source is given a playable audio track.
	silent := filepath.Join(dir, "silent.mp4")
	if err := run(t.Context(), os.Stderr, "ffmpeg", "-v", "error", "-i", src, "-an", "-c:v", "copy", silent); err != nil {
		t.Fatal(err)
	}
	p := writeManifest(t, dir, []Cut{{ID: "silent", Source: silent, End: 1}})
	if err := Prepare(t.Context(), p, filepath.Join(dir, "silent-out"), Options{Standard: "ntsc", Log: os.Stderr}); err != nil {
		t.Fatal(err)
	}
	info, err := inspect(t.Context(), filepath.Join(dir, "silent-out", "clips", "silent.mp4"))
	if err != nil || !info.Audio {
		t.Fatalf("missing silent audio: %v", err)
	}
	p = writeManifest(t, dir, []Cut{{ID: "past-end", Source: src, End: 99}})
	out := filepath.Join(dir, "bad-out")
	if err := Prepare(t.Context(), p, out, Options{Standard: "ntsc"}); err == nil {
		t.Fatal("accepted past-end cut")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("published failed render")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Prepare(ctx, p, out, Options{Standard: "ntsc"}); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err := inspect(t.Context(), "missing"); err == nil {
		t.Fatal("probed nonexistent media")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Fatal(err)
	}
	// Exercise the real downloader against a local HTTP origin, with no internet.
	srv := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer srv.Close()
	p = writeManifest(t, dir, []Cut{{ID: "web-one", Source: srv.URL + "/source.mp4", End: 1}, {ID: "web-two", Source: srv.URL + "/source.mp4", Start: 1, End: 2}})
	webOut := filepath.Join(dir, "web-out")
	if err := Prepare(t.Context(), p, webOut, Options{Standard: "ntsc", Log: os.Stderr}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(webOut)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatal("download scratch leaked into library")
	}
	retained, err := source(t.Context(), p, srv.URL+"/source.mp4", os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(retained)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(retained), "source.info.json")); err != nil {
		t.Fatalf("missing download provenance: %v", err)
	}
	srv.Close()
	t.Run("offline-reprocessing", func(t *testing.T) {
		// Keep the real render tools available, but remove yt-dlp from PATH.
		tools := t.TempDir()
		for _, name := range []string{"ffmpeg", "ffprobe"} {
			path, err := exec.LookPath(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(path, filepath.Join(tools, name)); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("PATH", tools)
		if err := Prepare(t.Context(), p, filepath.Join(dir, "web-reprocessed"), Options{Standard: "pal", Noise: 0, Log: os.Stderr}); err != nil {
			t.Fatalf("reprocessing required network or downloader: %v", err)
		}
		if got, err := os.ReadFile(retained); err != nil || !bytes.Equal(original, got) {
			t.Fatalf("reprocessing changed retained source: %v", err)
		}
		pastEnd := writeManifest(t, dir, []Cut{{ID: "too-long", Source: srv.URL + "/source.mp4", End: 99}})
		if err := Prepare(t.Context(), pastEnd, filepath.Join(dir, "web-failed"), Options{Standard: "ntsc"}); err == nil {
			t.Fatal("accepted past-end cached source")
		}
		if got, err := os.ReadFile(retained); err != nil || !bytes.Equal(original, got) {
			t.Fatalf("failed preparation changed retained source: %v", err)
		}
		// Corrupt retained input must fail without contacting the origin or
		// silently replacing the user's only remaining source.
		damaged := []byte("damaged retained video")
		if err := os.WriteFile(retained, damaged, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := source(t.Context(), p, srv.URL+"/source.mp4", io.Discard); err == nil || !strings.Contains(err.Error(), "retained source") {
			t.Fatalf("missing retained-source corruption error: %v", err)
		}
		if got, err := os.ReadFile(retained); err != nil || !bytes.Equal(damaged, got) {
			t.Fatalf("corrupt source was removed or replaced: %v", err)
		}
	})
	// A missing source must clean up its staging directory.
	p = writeManifest(t, dir, []Cut{{ID: "missing", Source: "missing.mp4", End: 1}})
	if err := Prepare(t.Context(), p, filepath.Join(dir, "missing-out"), Options{Standard: "ntsc"}); err == nil {
		t.Fatal("accepted missing source")
	}
	if err := Prepare(t.Context(), p, filepath.Join(dir, "absent-parent", "out"), Options{Standard: "ntsc"}); err == nil {
		t.Fatal("accepted absent output parent")
	}
	p = writeManifest(t, dir, []Cut{{ID: "unsupported", Source: "ftp://example.com/a", End: 1}})
	if err := Prepare(t.Context(), p, filepath.Join(dir, "ftp-out"), Options{Standard: "ntsc"}); err == nil {
		t.Fatal("accepted unsupported protocol")
	}
	audioFile := filepath.Join(dir, "audio.m4a")
	if err := run(t.Context(), os.Stderr, "ffmpeg", "-v", "error", "-i", src, "-vn", "-c:a", "copy", audioFile); err != nil {
		t.Fatal(err)
	}
	if _, err := inspect(t.Context(), audioFile); err == nil {
		t.Fatal("accepted audio-only input")
	}
}
