package media

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A square in the source must remain square while the colored background reaches
// every edge. This catches letterboxing, stretching, and incorrect SAR handling.
func TestFramingIntegration(t *testing.T) {
	if os.Getenv("PI_CRT_INTEGRATION") != "1" {
		t.Skip("set PI_CRT_INTEGRATION=1; requires FFmpeg and FFprobe")
	}
	for _, tc := range []struct {
		name, size, sar string
		boxWidth        int
		padding         string
		crop            *Crop
	}{
		{name: "widescreen", size: "960x540", sar: "1", boxWidth: 108},
		{name: "portrait", size: "540x960", sar: "1", boxWidth: 108},
		{name: "four-three", size: "720x540", sar: "1", boxWidth: 108},
		{name: "anamorphic", size: "720x540", sar: "4/3", boxWidth: 81},
		{name: "embedded-bars", size: "960x540", sar: "1", boxWidth: 108, padding: ",pad=1000:600:20:30", crop: &Crop{X: 20, Y: 30, Width: 960, Height: 540}},
		{name: "odd-crop", size: "960x540", sar: "1", boxWidth: 108, padding: ",format=yuv444p,pad=1000:600:19:29", crop: &Crop{X: 19, Y: 29, Width: 961, Height: 541}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "source.mkv")
			filter := fmt.Sprintf("color=c=green:s=%s:r=25:d=1,drawbox=x=(iw-%d)/2:y=(ih-108)/2:w=%d:h=108:color=blue:t=fill,setsar=%s", tc.size, tc.boxWidth, tc.boxWidth, tc.sar)
			filter += tc.padding
			if err := run(t.Context(), os.Stderr, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", filter, "-c:v", "ffv1", src); err != nil {
				t.Fatal(err)
			}
			manifest := writeManifest(t, dir, []Cut{{ID: "frame", Source: src, End: 1, Crop: tc.crop}})
			out := filepath.Join(dir, "out")
			if err := Prepare(t.Context(), manifest, out, Options{Standard: "ntsc", Log: os.Stderr}); err != nil {
				t.Fatal(err)
			}
			b, err := exec.CommandContext(t.Context(), "ffmpeg", "-v", "error", "-ss", "0.8", "-i", filepath.Join(out, "clips", "frame.mp4"), "-frames:v", "1", "-f", "image2pipe", "-c:v", "png", "-").Output()
			if err != nil {
				t.Fatal(err)
			}
			frame, err := png.Decode(bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			if frame.Bounds().Dx() != 320 || frame.Bounds().Dy() != 240 {
				t.Fatalf("wrong frame size: %v", frame.Bounds())
			}
			for _, p := range []image.Point{{2, 2}, {317, 2}, {2, 237}, {317, 237}, {160, 2}, {160, 237}, {2, 120}, {317, 120}} {
				r, g, b, _ := frame.At(p.X, p.Y).RGBA()
				if g < 90*257 || r > 40*257 || b > 40*257 {
					t.Errorf("picture does not fill edge %v: rgb(%d,%d,%d)", p, r/257, g/257, b/257)
				}
			}
			var square image.Rectangle
			for y := range 240 {
				for x := range 320 {
					r, g, b, _ := frame.At(x, y).RGBA()
					if b > 180*257 && r < 40*257 && g < 40*257 {
						square = square.Union(image.Rect(x, y, x+1, y+1))
					}
				}
			}
			if square.Dx() < 40 || square.Dy() < 40 || abs(square.Dx()-square.Dy()) > 3 || abs(square.Min.X+square.Max.X-320) > 4 || abs(square.Min.Y+square.Max.Y-240) > 4 {
				t.Fatalf("center square distorted or displaced: %v", square)
			}
		})
	}
}

func TestCropBoundsIntegration(t *testing.T) {
	if os.Getenv("PI_CRT_INTEGRATION") != "1" {
		t.Skip("set PI_CRT_INTEGRATION=1; requires FFmpeg and FFprobe")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mkv")
	if err := run(t.Context(), os.Stderr, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=s=320x240:d=1", "-c:v", "ffv1", src); err != nil {
		t.Fatal(err)
	}
	for _, crop := range []Crop{
		{X: -1, Width: 320, Height: 240},
		{Y: -1, Width: 320, Height: 240},
		{Width: 0, Height: 240},
		{Width: 320, Height: -1},
		{Width: 321, Height: 240},
		{Width: 320, Height: 241},
		{X: 1, Width: 320, Height: 240},
		{Y: 1, Width: 320, Height: 240},
		{X: int(^uint(0) >> 1), Width: 2, Height: 2},
	} {
		manifest := writeManifest(t, dir, []Cut{{ID: "bad", Source: src, End: 1, Crop: &crop}})
		out := filepath.Join(dir, "out")
		if err := Prepare(t.Context(), manifest, out, Options{Standard: "ntsc"}); err == nil || !strings.Contains(err.Error(), "crop") {
			t.Fatalf("crop %v: expected crop error, got %v", crop, err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.Name() == "out" || strings.HasPrefix(entry.Name(), ".pi-crt-prepare-") {
				t.Fatalf("invalid crop published output or leaked scratch: %s", entry.Name())
			}
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
