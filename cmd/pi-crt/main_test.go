package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func executeCommand(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	cmd := newRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

func TestLibraryCommands(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.mp4"), []byte("file availability test"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "library.json"), []byte(`{"standard":"ntsc","clips":[{"id":"a","group":"g","file":"a.mp4","duration":1}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"0", "1"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("AGENT", mode)
			out, err := executeCommand("media", "check", dir)
			if err != nil || !strings.Contains(out, `"id":"a"`) {
				t.Fatalf("check: %v %s", err, out)
			}
			out, err = executeCommand("player", "order", dir, "--count=3")
			if err != nil || out != "a\na\na\n" {
				t.Fatalf("order: %v %q", err, out)
			}
		})
	}
	if _, err := executeCommand("player", "order", dir, "--recent=-1"); err == nil {
		t.Fatal("accepted negative recent")
	}
	closed, err := os.Create(filepath.Join(dir, "closed"))
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"media", "check", dir}, {"player", "order", dir}} {
		c := newRootCommand()
		c.SetArgs(args)
		c.SetOut(closed)
		if err := c.Execute(); err == nil {
			t.Fatalf("ignored broken output: %v", args)
		}
	}
}

func TestPrepareCommandIntegration(t *testing.T) {
	if os.Getenv("PI_CRT_INTEGRATION") != "1" {
		t.Skip("requires FFmpeg")
	}
	dir := t.TempDir()
	cmd := exec.CommandContext(t.Context(), "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=160x120:duration=1", "-c:v", "libx264", filepath.Join(dir, "in.mp4"))
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, b)
	}
	p := filepath.Join(dir, "cuts.json")
	if err := os.WriteFile(p, []byte(`{"clips":[{"id":"a","source":"in.mp4","end":0.5}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	result, err := executeCommand("media", "prepare", p, out)
	if err != nil || !strings.Contains(result, out) {
		t.Fatalf("prepare: %v %s", err, result)
	}
}

func TestRootCommand_HelpAndVersion(t *testing.T) {
	t.Setenv("AGENT", "0")
	out, err := executeCommand("--help")
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected help output")
	}

	outVer, errVer := executeCommand("--version")
	if errVer != nil {
		t.Fatalf("version failed: %v", errVer)
	}
	if outVer != version+"\n" {
		t.Errorf("expected '%s\\n', got %q", version, outVer)
	}
}

func TestCommandValidation(t *testing.T) {
	for _, args := range [][]string{{"media", "prepare"}, {"media", "prepare", "missing", "out"}, {"media", "check", "missing"}, {"player", "run", "missing"}, {"player", "order", "missing"}, {"player", "order", "missing", "--count=0"}, {"unknown"}} {
		if _, err := executeCommand(args...); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, args := range [][]string{{"media"}, {"player"}, {"help", "player"}, {"completion", "bash"}} {
		out, err := executeCommand(args...)
		if err != nil || strings.TrimSpace(out) == "" {
			t.Fatalf("%v: %v", args, err)
		}
	}
}
