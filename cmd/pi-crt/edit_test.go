package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditCommandValidation(t *testing.T) {
	for _, args := range [][]string{{"media", "edit"}, {"media", "trim"}, {"media", "edit", "missing", "id"}, {"media", "trim", "missing", "id", "missing.csv"}} {
		if _, err := executeCommand(args...); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestEditCommandsIntegration(t *testing.T) {
	if os.Getenv("PI_CRT_INTEGRATION") != "1" {
		t.Skip("requires FFmpeg")
	}
	dir := t.TempDir()
	if b, err := exec.CommandContext(t.Context(), "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=160x120:d=2", "-c:v", "libx264", filepath.Join(dir, "source.mp4")).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, b)
	}
	manifest, csv := filepath.Join(dir, "manifest.json"), filepath.Join(dir, "trim.csv")
	if err := os.WriteFile(manifest, []byte(`{"clips":[{"id":"a","source":"source.mp4","end":2}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(csv, []byte("0.5,1.5,keep\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"0", "1"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("AGENT", mode)
			out, err := executeCommand("media", "edit", manifest, "a")
			if err != nil || !strings.HasSuffix(out, "a-proj.llc\n") {
				t.Fatalf("edit: %q %v", out, err)
			}
			out, err = executeCommand("media", "trim", manifest, "a", csv)
			if err != nil || out != manifest+"\n" {
				t.Fatalf("trim: %q %v", out, err)
			}
		})
	}
	b, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Clips []struct{ Start, End float64 }
	}
	if err := json.Unmarshal(b, &m); err != nil || m.Clips[0].Start != 0.5 || m.Clips[0].End != 1.5 {
		t.Fatalf("timestamps not saved: %s %v", b, err)
	}
	closed, err := os.Create(filepath.Join(dir, "closed"))
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"media", "edit", manifest, "a"}, {"media", "trim", manifest, "a", csv}} {
		cmd := newRootCommand()
		cmd.SetArgs(args)
		cmd.SetOut(closed)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("ignored output error: %v", args)
		}
	}
}
