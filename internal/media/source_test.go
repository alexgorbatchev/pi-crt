package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYouTubeSourceQualityIntegration(t *testing.T) {
	if os.Getenv("PI_CRT_INTEGRATION") != "1" {
		t.Skip("set PI_CRT_INTEGRATION=1; requires FFmpeg and FFprobe")
	}
	dir := t.TempDir()
	for _, tc := range []struct {
		name, size string
		audio      bool
		valid      bool
	}{
		{name: "720p-with-audio", size: "1280x720", audio: true, valid: true},
		{name: "lower-resolution", size: "640x360", audio: true},
		{name: "missing-audio", size: "1280x720"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".mkv")
			args := []string{"-v", "error", "-f", "lavfi", "-i", "color=s=" + tc.size + ":d=0.5"}
			if tc.audio {
				args = append(args, "-f", "lavfi", "-i", "sine=duration=0.5", "-c:a", "aac")
			}
			args = append(args, "-c:v", "libx264", path)
			if err := run(t.Context(), os.Stderr, "ffmpeg", args...); err != nil {
				t.Fatal(err)
			}
			err := validateDownload(t.Context(), path, true, os.Stderr)
			if tc.valid && err != nil {
				t.Fatal(err)
			}
			if !tc.valid && (err == nil || !strings.Contains(err.Error(), "720-line video and audio")) {
				t.Fatalf("accepted unsuitable YouTube master: %v", err)
			}
			// Other supported URLs may still supply smaller or silent sources.
			if err := validateDownload(t.Context(), path, false, os.Stderr); err != nil {
				t.Fatalf("rejected usable generic source: %v", err)
			}
		})
	}
}
