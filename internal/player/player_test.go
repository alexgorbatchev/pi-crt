package player

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"pi-crt/internal/library"
)

func TestHTTPPlaybackProtocol(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "" || p != "secret" {
			t.Error("missing VLC basic auth")
		}
		if r.URL.Path != "/requests/status.json" {
			t.Error("wrong endpoint")
		}
		i := requests.Add(1)
		switch i {
		case 1:
			if r.URL.Query().Get("command") != "pl_empty" {
				t.Error("playlist must be bounded")
			}
		case 2:
			q := r.URL.Query()
			if q.Get("command") != "in_play" {
				t.Error("missing play command")
			}
			u, err := url.Parse(q.Get("input"))
			if err != nil || u.Scheme != "file" || u.Path != "/test & media/a #.mp4" {
				t.Errorf("bad MRL: %v %v", u, err)
			}
			if _, err := fmt.Fprint(w, `{"state":"playing","stats":{"decodedvideo":1,"displayedpictures":1}}`); err != nil {
				t.Error(err)
			}
			return
		}
		if _, err := fmt.Fprint(w, `{"state":"stopped"}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	c := Client{URL: server.URL, Password: "secret", HTTP: server.Client()}
	if err := c.Play(t.Context(), "/test & media/a #.mp4", 1); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 3 {
		t.Fatal("unexpected protocol sequence")
	}
}

func TestHTTPErrorsAndCancellation(t *testing.T) {
	for _, response := range []string{"unauthorized", "bad-json", "playing", "paused"} {
		t.Run(response, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if response == "unauthorized" {
					w.WriteHeader(401)
					return
				}
				if response == "bad-json" {
					if _, err := fmt.Fprint(w, "{"); err != nil {
						t.Error(err)
					}
					return
				}
				if _, err := fmt.Fprintf(w, `{"state":%q}`, response); err != nil {
					t.Error(err)
				}
			}))
			defer s.Close()
			c := Client{URL: s.URL, HTTP: s.Client()}
			ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
			defer cancel()
			if err := c.Play(ctx, "/test.mp4", 1); err == nil {
				t.Fatal("accepted invalid response or ignored cancellation")
			}
		})
	}
	c := Client{URL: "://bad", HTTP: http.DefaultClient}
	if _, err := c.Request(t.Context(), nil); err == nil {
		t.Fatal("accepted URL")
	}
	c.URL = "http://127.0.0.1:0"
	if _, err := c.Request(t.Context(), nil); err == nil {
		t.Fatal("accepted connection failure")
	}
}

func TestRunValidation(t *testing.T) {
	for _, o := range []Options{{Port: 1}, {Port: 70000}, {Port: 42121, Count: -1}, {Port: 42121}} {
		if err := Run(t.Context(), "missing", o); err == nil {
			t.Fatal("accepted invalid setup")
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "clip.mp4"), []byte("not a video"), 0600); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(library.Library{Standard: "ntsc", Clips: []library.Clip{{ID: "a", Group: "g", File: "clip.mp4", Duration: 1}}})
	if err := os.WriteFile(filepath.Join(dir, "library.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Run(t.Context(), dir, Options{Port: 42121, Recent: -1}); err == nil {
		t.Fatal("accepted negative window")
	}
	if err := Run(t.Context(), dir, Options{Port: 42121, VLC: "/nonexistent/vlc"}); err == nil {
		t.Fatal("accepted nonexistent VLC")
	}
	if err := Run(t.Context(), dir, Options{Port: 42121, VLC: "false"}); err == nil || !strings.Contains(err.Error(), "before HTTP startup") {
		t.Fatalf("missing early process exit: %v", err)
	}
}

func TestVLCIntegration(t *testing.T) {
	vlc := os.Getenv("PI_CRT_VLC")
	if vlc == "" {
		t.Skip("set PI_CRT_VLC to run a real VLC integration test")
	}
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.mp4")
	cmd := exec.CommandContext(t.Context(), "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x240:duration=1", "-c:v", "libx264", clip)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate clip: %v %s", err, b)
	}
	b, _ := json.Marshal(library.Library{Standard: "ntsc", Clips: []library.Clip{{ID: "a", Group: "g", File: "clip.mp4", Duration: 1}}})
	if err := os.WriteFile(filepath.Join(dir, "library.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	headlessArgs := []string{"--vout=vdummy", "--aout=adummy", "--avcodec-hw=none"}
	if err := Run(ctx, dir, Options{Port: port, VLC: vlc, Count: 3, Recent: 5, VLCArgs: headlessArgs, Log: os.Stderr}); err != nil {
		t.Fatal(err)
	}
	// A broken file must not stop a usable library or spin in a retry loop.
	bad := filepath.Join(dir, "bad.mp4")
	if err := os.WriteFile(bad, []byte("not video"), 0600); err != nil {
		t.Fatal(err)
	}
	lib := library.Library{Standard: "ntsc", Clips: []library.Clip{{ID: "a", Group: "g", File: "clip.mp4", Duration: 1}, {ID: "bad", Group: "b", File: "bad.mp4", Duration: 1}}}
	b, _ = json.Marshal(lib)
	if err := os.WriteFile(filepath.Join(dir, "library.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Run(t.Context(), dir, Options{Port: port, VLC: vlc, Count: 2, Recent: 5, VLCArgs: headlessArgs, Log: os.Stderr}); err != nil {
		t.Fatal(err)
	}
	lib.Clips = lib.Clips[1:]
	b, _ = json.Marshal(lib)
	if err := os.WriteFile(filepath.Join(dir, "library.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Run(t.Context(), dir, Options{Port: port, VLC: vlc, Count: 1, VLCArgs: headlessArgs, Log: os.Stderr}); err == nil || !strings.Contains(err.Error(), "all clips failed") {
		t.Fatalf("did not fail broken library: %v", err)
	}
	ctx, cancel = context.WithCancel(t.Context())
	cancel()
	if err := Run(ctx, dir, Options{Port: port, VLC: vlc, Log: io.Discard}); err == nil {
		t.Fatal("ignored cancellation")
	}
}
