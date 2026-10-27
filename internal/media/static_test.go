package media

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Decode the actual H.264 output: encoding must preserve the coarse static
// blocks, their animation, and the fade at both supported frame rates.
func TestStaticIntegration(t *testing.T) {
	if os.Getenv("PI_CRT_INTEGRATION") != "1" {
		t.Skip("set PI_CRT_INTEGRATION=1; requires FFmpeg and FFprobe")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "gray.mkv")
	if err := run(t.Context(), os.Stderr, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=gray:s=320x240:r=25:d=1", "-c:v", "ffv1", src); err != nil {
		t.Fatal(err)
	}
	manifest := writeManifest(t, dir, []Cut{{ID: "static", Source: src, End: 1}})
	for _, standard := range []string{"ntsc", "pal"} {
		t.Run(standard, func(t *testing.T) {
			out := filepath.Join(dir, standard)
			if err := Prepare(t.Context(), manifest, out, Options{Standard: standard, Noise: 0, Log: os.Stderr}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(out, "clips", "static.mp4")
			first := decodeGray240(t, path, 0.08)
			second := decodeGray240(t, path, 0.20)
			fading := decodeGray240(t, path, 0.36)
			trailer := decodeGray240(t, path, 0.70)
			var movement, level, faded float64
			for i := range first {
				movement += float64(abs(int(first[i]) - int(second[i])))
				level += float64(first[i])
				faded += float64(fading[i])
			}
			if movement/float64(len(first)) < 20 {
				t.Fatal("static is frozen or lost its contrast in the encoded output")
			}
			if faded >= level*0.9 {
				t.Fatal("static did not fade before the trailer")
			}
			for _, frame := range [][]byte{first, second} {
				var edge, interior float64
				var edges, interiors int
				for y := range 240 {
					for x := 1; x < 320; x++ {
						d := float64(abs(int(frame[y*320+x]) - int(frame[y*320+x-1])))
						if x%4 == 0 {
							edge += d
							edges++
						} else {
							interior += d
							interiors++
						}
					}
				}
				edge /= float64(edges)
				interior /= float64(interiors)
				if edge < 20 || edge < 3*interior {
					t.Fatalf("static lacks coarse blocks at 240 lines: boundary difference %.2f, interior %.2f", edge, interior)
				}
			}
			for _, value := range trailer {
				if value < 120 || value > 135 {
					t.Fatalf("static leaked into noise-disabled trailer: gray value %d", value)
				}
			}
		})
	}
}

func decodeGray240(t *testing.T, path string, seconds float64) []byte {
	t.Helper()
	frame, err := exec.CommandContext(t.Context(), "ffmpeg", "-v", "error", "-ss", fmt.Sprint(seconds), "-i", path, "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "gray", "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != 320*240 {
		t.Fatalf("unexpected decoded frame length: %d", len(frame))
	}
	return frame
}
