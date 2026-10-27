package media

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Decode the actual H.264 output: encoding must preserve the coarse static
// blocks, their animation, and a direct dissolve at both supported frame rates.
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
			dissolving := decodeGray240(t, path, 0.50)
			trailer := decodeGray240(t, path, 0.70)
			var movement, level, faded, staticContrast, dissolveContrast float64
			for i := range first {
				movement += float64(abs(int(first[i]) - int(second[i])))
				level += float64(first[i])
				faded += float64(fading[i])
				staticContrast += float64(abs(int(first[i]) - 128))
				dissolveContrast += float64(abs(int(dissolving[i]) - 128))
			}
			if movement/float64(len(first)) < 20 {
				t.Fatal("static is frozen or lost its contrast in the encoded output")
			}
			if faded < level*0.9 {
				t.Fatal("static faded to black before the trailer")
			}
			if dissolveContrast < float64(len(first))*5 || dissolveContrast > staticContrast*0.8 {
				t.Fatal("static did not partially dissolve into the trailer picture")
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

func TestTapeEffectsIntegration(t *testing.T) {
	if os.Getenv("PI_CRT_INTEGRATION") != "1" {
		t.Skip("set PI_CRT_INTEGRATION=1; requires FFmpeg and FFprobe")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "opening.mkv")
	filter := "color=c=gray:s=320x240:r=25:d=6,drawbox=color=black:t=fill:enable='lt(t,0.6)+between(t,4,4.4)'"
	if err := run(t.Context(), os.Stderr, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", filter, "-c:v", "ffv1", src); err != nil {
		t.Fatal(err)
	}
	for _, standard := range []string{"ntsc", "pal"} {
		t.Run(standard, func(t *testing.T) {
			manifest := writeManifest(t, dir, []Cut{{ID: "opening", Source: src, End: 6}, {ID: "picture", Source: src, Start: 0.8, End: 1.8}})
			out := filepath.Join(dir, standard)
			if err := Prepare(t.Context(), manifest, out, Options{Standard: standard, Log: os.Stderr}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(out, "clips", "opening.mp4")
			first := decodeGray240(t, path, 0.08)
			other := decodeGray240(t, filepath.Join(out, "clips", "picture.mp4"), 0.08)
			var difference int
			for i := range first {
				difference += abs(int(first[i]) - int(other[i]))
			}
			if float64(difference)/float64(len(first)) > 5 {
				t.Fatal("clips do not start with the same static pattern")
			}
			for _, seconds := range []float64{0.36, 0.44, 0.8, 1.04, 1.12, 1.3} {
				frame := decodeGray240(t, path, seconds)
				var level int
				for _, v := range frame {
					level += int(v)
				}
				if float64(level)/float64(len(frame)) < 100 {
					t.Errorf("black gap at %.2fs", seconds)
				}
			}
			previousRow := -1
			for _, seconds := range []float64{1.9, 2.0, 3.6} {
				frame := decodeGray240(t, path, seconds)
				rows, firstRow := 0, -1
				for y := range 240 {
					changed := 0
					for _, v := range frame[y*320 : (y+1)*320] {
						if abs(int(v)-128) > 30 {
							changed++
						}
					}
					if changed > 120 {
						if firstRow < 0 {
							firstRow = y
						}
						rows++
					}
				}
				if rows < 2 || rows > 12 {
					t.Errorf("tracking burst at %.2fs affects %d rows; want a visible localized band", seconds, rows)
				}
				if firstRow == previousRow {
					t.Errorf("tracking band did not move at %.2fs", seconds)
				}
				previousRow = firstRow
			}
			for _, seconds := range []float64{1.4, 2.5, 5.4} {
				for _, v := range decodeGray240(t, path, seconds) {
					if v < 120 || v > 135 {
						t.Fatalf("tracking lines persisted outside a burst at %.2fs: %d", seconds, v)
					}
				}
			}
			for _, v := range decodeGray240(t, path, 4.6) {
				if v > 8 {
					t.Fatal("static covered an intentional later black interval")
				}
			}
			info, err := inspect(t.Context(), path)
			if err != nil || info.Duration < 6.35 || info.Duration > 6.5 {
				t.Fatalf("opening effect changed retained duration: %+v, %v", info, err)
			}
		})
	}
	for _, tc := range []struct {
		name string
		cut  Cut
		want float64
	}{
		{"opening", Cut{End: 6}, 0.6},
		{"all-black", Cut{End: 0.4}, 0.4},
		{"nonzero-start", Cut{Start: 0.8, End: 6}, 0},
		{"black-at-cut", Cut{Start: 4, End: 6}, 0.44},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := leadingBlack(t.Context(), src, tc.cut, pictureFilter(tc.cut, "25"))
			if err != nil || got < tc.want-0.01 || got > tc.want+0.01 {
				t.Fatalf("opening duration = %f, %v; want %f", got, err, tc.want)
			}
		})
	}
	if _, err := leadingBlack(t.Context(), "missing", Cut{End: 1}, "null"); err == nil {
		t.Fatal("opening inspection accepted a missing source")
	}
	manifest := writeManifest(t, dir, []Cut{{ID: "black", Source: src, End: 0.5}})
	out := filepath.Join(dir, "black")
	if err := Prepare(t.Context(), manifest, out, Options{Standard: "pal"}); err != nil {
		t.Fatal(err)
	}
	for _, v := range decodeGray240(t, filepath.Join(out, "clips", "black.mp4"), 0.8) {
		if v > 180 {
			return
		}
	}
	t.Fatal("entirely black cut lost static before its end")
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
