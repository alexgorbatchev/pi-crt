package player

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDecodedFramesWithoutDisplayAreFailure(t *testing.T) {
	var requests atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		state := "stopped"
		if requests.Add(1) == 2 {
			state = "playing"
		}
		if _, err := fmt.Fprintf(w, `{"state":%q,"stats":{"decodedvideo":10,"displayedpictures":0}}`, state); err != nil {
			t.Error(err)
		}
	}))
	defer s.Close()
	c := Client{URL: s.URL, HTTP: s.Client()}
	if err := c.Play(t.Context(), "/decoded-without-output.mp4", 1); err == nil || !strings.Contains(err.Error(), "displaying") {
		t.Fatalf("reported successful playback without output: %v", err)
	}
}
