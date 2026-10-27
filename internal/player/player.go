// Package player controls one persistent VLC process through its loopback HTTP API.
package player

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"pi-crt/internal/library"
	"pi-crt/internal/schedule"
)

type Options struct {
	VLC     string
	Port    int
	Recent  int
	Count   int
	VLCArgs []string
	Log     io.Writer
}

type Status struct {
	State string `json:"state"`
	Stats struct {
		DecodedVideo      int `json:"decodedvideo"`
		DisplayedPictures int `json:"displayedpictures"`
	} `json:"stats"`
}

type Client struct {
	URL      string
	Password string
	HTTP     *http.Client
}

func (c Client) Request(ctx context.Context, values url.Values) (status Status, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL+"/requests/status.json?"+values.Encode(), nil)
	if err != nil {
		return status, err
	}
	req.SetBasicAuth("", c.Password)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return status, err
	}
	defer func() { err = errors.Join(err, resp.Body.Close()) }()
	if resp.StatusCode != http.StatusOK {
		return status, fmt.Errorf("VLC HTTP status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&status); err != nil {
		return status, fmt.Errorf("VLC status: %w", err)
	}
	return status, nil
}

// Play bounds the playlist to one item. Playback is sequential, not guaranteed gapless.
func (c Client) Play(ctx context.Context, path string, duration float64) error {
	if _, err := c.Request(ctx, url.Values{"command": {"pl_empty"}}); err != nil {
		return err
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	status, err := c.Request(ctx, url.Values{"command": {"in_play"}, "input": {u.String()}})
	if err != nil {
		return err
	}
	started := status.State == "playing"
	decoded := status.Stats.DecodedVideo > 0
	displayed := status.Stats.DisplayedPictures > 0
	startDeadline := time.Now().Add(15 * time.Second)
	deadline := time.Now().Add(time.Duration(duration*float64(time.Second)) + 30*time.Second)
	for {
		if err := wait(ctx, 100*time.Millisecond); err != nil {
			return err
		}
		status, err = c.Request(ctx, nil)
		if err != nil {
			return err
		}
		if status.State == "playing" {
			started = true
		}
		decoded = decoded || status.Stats.DecodedVideo > 0
		displayed = displayed || status.Stats.DisplayedPictures > 0
		if started && status.State == "stopped" {
			if !decoded || !displayed {
				return fmt.Errorf("VLC stopped without decoding and displaying video: %s", path)
			}
			return nil
		}
		if !started && time.Now().After(startDeadline) {
			return fmt.Errorf("VLC did not start %s within 15s", path)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("VLC stalled playing %s", path)
		}
	}
}

func Run(ctx context.Context, dir string, opts Options) error {
	if opts.Port < 1024 || opts.Port > 65535 || opts.Count < 0 {
		return fmt.Errorf("port must be 1024..65535 and count must be nonnegative")
	}
	lib, err := library.Load(dir)
	if err != nil {
		return err
	}
	bag, err := schedule.New(lib.Clips, opts.Recent)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	password := rand.Text()
	client := Client{URL: "http://127.0.0.1:" + strconv.Itoa(opts.Port), Password: password, HTTP: &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}}
	defer client.HTTP.CloseIdleConnections()
	processCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	args := []string{"--ignore-config", "--intf=dummy", "--extraintf=http", "--http-host=127.0.0.1", "--http-port=" + strconv.Itoa(opts.Port), "--http-password=" + password, "--no-video-title-show", "--no-osd", "--no-media-library", "--no-loop", "--no-repeat", "--no-random", "--fullscreen"}
	args = append(args, opts.VLCArgs...)
	proc := exec.CommandContext(processCtx, opts.VLC, args...)
	proc.Stdout, proc.Stderr = opts.Log, opts.Log
	if err := proc.Start(); err != nil {
		return fmt.Errorf("start VLC: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- proc.Wait() }()
	defer func() { cancel(); <-done }()
	readyCtx, readyCancel := context.WithTimeout(ctx, 20*time.Second)
	defer readyCancel()
	for {
		if _, err := client.Request(readyCtx, nil); err == nil {
			break
		}
		select {
		case err := <-done:
			done <- err
			return fmt.Errorf("VLC exited before HTTP startup: %v", err)
		default:
		}
		if err := wait(readyCtx, 100*time.Millisecond); err != nil {
			return fmt.Errorf("wait for VLC: %w", err)
		}
	}
	failed := make(map[string]bool)
	played := 0
	for opts.Count == 0 || played < opts.Count {
		if err := ctx.Err(); err != nil {
			return err
		}
		clip := bag.Next()
		if opts.Log != nil {
			if _, err := fmt.Fprintf(opts.Log, "Playing %s\n", clip.ID); err != nil {
				return err
			}
		}
		if err := client.Play(ctx, filepath.Join(abs, clip.File), clip.Duration); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			failed[clip.ID] = true
			if opts.Log != nil {
				if _, logErr := fmt.Fprintf(opts.Log, "Skipping %s until restart: %v\n", clip.ID, err); logErr != nil {
					return logErr
				}
			}
			if len(failed) == len(lib.Clips) {
				return fmt.Errorf("all clips failed; check VLC output and media")
			}
			if err := bag.Exclude(clip.ID); err != nil {
				return err
			}
			continue
		}
		played++
	}
	return nil
}

func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
