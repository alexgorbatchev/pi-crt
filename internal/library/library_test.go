package library

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "clip.mp4")
	if err := os.WriteFile(file, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	base := Library{Standard: "ntsc", Clips: []Clip{{ID: "a", Group: "g", File: "clip.mp4", Duration: 1}}}
	cases := []struct {
		name   string
		change func(*Library)
		valid  bool
	}{
		{"valid", func(*Library) {}, true},
		{"pal", func(l *Library) { l.Standard = "pal" }, true},
		{"bad-standard", func(l *Library) { l.Standard = "what" }, false},
		{"empty", func(l *Library) { l.Clips = nil }, false},
		{"duplicate", func(l *Library) { l.Clips = append(l.Clips, l.Clips[0]) }, false},
		{"escape", func(l *Library) { l.Clips[0].File = "../outside" }, false},
		{"missing", func(l *Library) { l.Clips[0].File = "missing" }, false},
		{"directory", func(l *Library) { l.Clips[0].File = "." }, false},
		{"duration", func(l *Library) { l.Clips[0].Duration = 0 }, false},
		{"group", func(l *Library) { l.Clips[0].Group = "" }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := base
			l.Clips = append([]Clip(nil), base.Clips...)
			tc.change(&l)
			data, _ := json.Marshal(l)
			if err := os.WriteFile(filepath.Join(dir, "library.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(dir)
			if (err == nil) != tc.valid {
				t.Fatalf("unexpected result: %v", err)
			}
		})
	}
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, file); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(base)
	if err := os.WriteFile(filepath.Join(dir, "library.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("followed symlink outside library")
	}
	if _, err := Load(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("accepted missing library")
	}
}

func TestReadJSONAndFiniteDuration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "json")
	for _, data := range []string{"{", `{"unknown":1}`, `{} {}`, `{} garbage`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		var l Library
		if err := ReadJSON(path, &l); err == nil {
			t.Fatalf("accepted %q", data)
		}
	}
	for _, v := range []float64{-1, 0, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if ValidDuration(v) {
			t.Fatalf("accepted %v", v)
		}
	}
}
