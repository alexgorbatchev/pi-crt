package media

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"pi-crt/internal/library"
)

func TestReadTrim(t *testing.T) {
	for _, input := range []string{"1.25,3.5,keep\n", "\ufeff1.25,3.5,\"label, with comma\",tag\r\n", "1.25,3.5\n"} {
		p := filepath.Join(t.TempDir(), "trim.csv")
		if err := os.WriteFile(p, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
		start, end, err := readTrim(p)
		if err != nil || start != 1.25 || end != 3.5 {
			t.Fatalf("%q: %v %v %v", input, start, end, err)
		}
	}
	for _, input := range []string{"", "1", "\"bad", "1,2\n3,4\n", "1,2\n\"bad", "start,end,label", "00:01,00:02,x", "1,,marker", "-1,2", "2,1", "1,1.1", "NaN,2", "1,NaN", "1,Inf", "Inf,Inf"} {
		t.Run(input, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "trim.csv")
			if err := os.WriteFile(p, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := readTrim(p); err == nil {
				t.Fatalf("accepted %q", input)
			}
		})
	}
	if _, _, err := readTrim(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("accepted missing CSV")
	}
	p := filepath.Join(t.TempDir(), "from-zero.csv")
	if err := os.WriteFile(p, []byte(",2,label\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if start, end, err := readTrim(p); err != nil || start != 0 || end != 2 {
		t.Fatalf("empty start should mean zero: %v %v %v", start, end, err)
	}
}

func TestEditValidation(t *testing.T) {
	dir := t.TempDir()
	manifest := writeManifest(t, dir, []Cut{{ID: "a", Source: "in.mp4", End: 2}})
	csv := filepath.Join(dir, "trim.csv")
	if err := os.WriteFile(csv, []byte("0,1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "missing"} {
		if _, err := EditProject(t.Context(), manifest, id); err == nil {
			t.Fatal("accepted missing source/cut")
		}
		if err := Trim(t.Context(), manifest, id, csv); err == nil {
			t.Fatal("accepted missing source/cut")
		}
	}
	if err := Trim(t.Context(), manifest, "a", "missing"); err == nil {
		t.Fatal("accepted missing CSV")
	}
	for _, src := range []string{"https://example.com/trailer", "http://%"} {
		p := writeManifest(t, dir, []Cut{{ID: "a", Source: src, End: 2}})
		if _, _, _, err := localCut(p, "a"); err == nil {
			t.Fatal("accepted remote or malformed source")
		}
	}
	if _, _, _, err := localCut("missing", "a"); err == nil {
		t.Fatal("accepted missing manifest")
	}
}

func TestReplaceManifest(t *testing.T) {
	dir := t.TempDir()
	m := Manifest{Clips: []Cut{{ID: "a", Source: "source.mp4", Start: 1, End: 2}}}
	p := writeManifest(t, dir, []Cut{{ID: "a", Source: "source.mp4", End: 3}})
	if err := replaceManifest(p, m); err != nil {
		t.Fatal(err)
	}
	var saved Manifest
	if err := library.ReadJSON(p, &saved); err != nil || saved.Clips[0].Start != 1 || saved.Clips[0].End != 2 {
		t.Fatalf("replacement: %+v %v", saved, err)
	}
	info, err := os.Stat(p)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions changed: %v %v", info, err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(p, link); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{link, dir, filepath.Join(dir, "missing")} {
		if err := replaceManifest(target, m); err == nil {
			t.Fatalf("accepted %s", target)
		}
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".pi-crt-trim-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("left scratch files: %v %v", matches, err)
	}
}

func TestEditAndTrimIntegration(t *testing.T) {
	if os.Getenv("PI_CRT_INTEGRATION") != "1" {
		t.Skip("requires FFmpeg")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "source #é's.mp4")
	args := []string{"-v", "error", "-f", "lavfi", "-i", "color=red:s=640x480:d=1:r=25", "-f", "lavfi", "-i", "color=lime:s=640x480:d=1:r=25", "-f", "lavfi", "-i", "color=blue:s=640x480:d=1:r=25", "-filter_complex", "[0:v][1:v][2:v]concat=n=3:v=1:a=0", "-c:v", "libx264", src}
	if b, err := exec.CommandContext(t.Context(), "ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, b)
	}
	original, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	crop := &Crop{Width: 640, Height: 480}
	manifest := writeManifest(t, dir, []Cut{{ID: "a", Group: "film", Source: filepath.Base(src), End: 3, Crop: crop}, {ID: "b", Group: "film", Source: src, Start: 0.2, End: 2.5}})
	project, err := EditProject(t.Context(), manifest, "a")
	if err != nil {
		t.Fatal(err)
	}
	var initial struct {
		MediaFileName string        `json:"mediaFileName"`
		Version       int           `json:"version"`
		CutSegments   []editSegment `json:"cutSegments"`
	}
	if b, err := os.ReadFile(project); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(b, &initial); err != nil || initial.Version != 2 || initial.MediaFileName != filepath.Base(src) || len(initial.CutSegments) != 1 || initial.CutSegments[0].End != 3 {
		t.Fatalf("native project does not match source: %+v %v", initial, err)
	}
	// Saved editor decisions must survive reopening, even though LLC is JSON5.
	decision := []byte("{version: 2, cutSegments: [{start: 1, end: 2}]}\n")
	if err := os.WriteFile(project, decision, 0600); err != nil {
		t.Fatal(err)
	}
	if reopened, err := EditProject(t.Context(), manifest, "a"); err != nil || reopened != project {
		t.Fatalf("reopen: %s %v", reopened, err)
	}
	if got, err := os.ReadFile(project); err != nil || !bytes.Equal(got, decision) {
		t.Fatalf("overwrote editor decisions: %v", err)
	}
	csv := filepath.Join(dir, "trim.csv")
	if err := os.WriteFile(csv, []byte("1,2,keep green\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Trim(t.Context(), manifest, "a", csv); err != nil {
		t.Fatal(err)
	}
	m, err := loadManifest(manifest)
	if err != nil || m.Clips[0].Start != 1 || m.Clips[0].End != 2 || m.Clips[0].Group != "film" || *m.Clips[0].Crop != *crop || m.Clips[1].Start != 0.2 || m.Clips[1].End != 2.5 {
		t.Fatalf("lost cuts or framing: %+v %v", m, err)
	}
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(csv, []byte("1,4\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Trim(t.Context(), manifest, "a", csv); err == nil || !strings.Contains(err.Error(), "beyond source") {
		t.Fatalf("accepted beyond EOF: %v", err)
	}
	after, err := os.ReadFile(manifest)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("failed edit changed manifest: %v", err)
	}
	if got, err := os.ReadFile(src); err != nil || !bytes.Equal(original, got) {
		t.Fatalf("changed original video: %v", err)
	}
	out := filepath.Join(dir, "prepared")
	if err := Prepare(t.Context(), manifest, out, Options{Standard: "pal"}); err != nil {
		t.Fatal(err)
	}
	prepared, err := inspect(t.Context(), filepath.Join(out, "clips", "a.mp4"))
	if err != nil || math.Abs(prepared.Duration-1.4) > 0.1 {
		t.Fatalf("outro retained or wrong duration: %+v %v", prepared, err)
	}
	if got, err := os.ReadFile(src); err != nil || !bytes.Equal(original, got) {
		t.Fatalf("preparation changed original video: %v", err)
	}
	// The retained second is green. Red intro and blue outro must be absent.
	frame, err := exec.CommandContext(t.Context(), "ffmpeg", "-v", "error", "-ss", "0.7", "-i", filepath.Join(out, "clips", "a.mp4"), "-frames:v", "1", "-vf", "scale=1:1", "-f", "rawvideo", "-pix_fmt", "rgb24", "-").Output()
	if err != nil || len(frame) != 3 || frame[1] < 220 || frame[0] > 25 || frame[2] > 25 {
		t.Fatalf("wrong retained footage: %v %v", frame, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Trim(ctx, manifest, "a", csv); err == nil {
		t.Fatal("ignored canceled probe")
	}
	if err := os.Remove(project); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, project); err != nil {
		t.Fatal(err)
	}
	if _, err := EditProject(t.Context(), manifest, "a"); err == nil {
		t.Fatal("accepted project symlink")
	}
}
