package media

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"pi-crt/internal/library"
)

type editSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Name  string  `json:"name"`
}

func loadManifest(path string) (m Manifest, err error) {
	if err := library.ReadJSON(path, &m); err != nil {
		return m, err
	}
	if len(m.Clips) == 0 {
		return m, fmt.Errorf("manifest has no clips")
	}
	seen := make(map[string]bool)
	for _, c := range m.Clips {
		if !library.ValidID(c.ID) || seen[c.ID] || c.Source == "" || c.Start < 0 || math.IsNaN(c.Start) || !library.ValidDuration(c.End-c.Start) || c.End-c.Start < 0.5 {
			return m, fmt.Errorf("invalid or duplicate cut %q; require source and at least 0.5 seconds", c.ID)
		}
		if r := c.Crop; r != nil && (r.X < 0 || r.Y < 0 || r.Width <= 0 || r.Height <= 0) {
			return m, fmt.Errorf("cut %s: crop requires nonnegative x/y and positive width/height", c.ID)
		}
		seen[c.ID] = true
	}
	return m, nil
}

func localCut(manifest, id string) (Manifest, int, string, error) {
	m, err := loadManifest(manifest)
	if err != nil {
		return m, 0, "", err
	}
	for i, c := range m.Clips {
		if c.ID != id {
			continue
		}
		u, err := url.Parse(c.Source)
		if err != nil || u.Scheme != "" {
			return m, 0, "", fmt.Errorf("cut %s: editing requires a local source file", id)
		}
		path := c.Source
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(manifest), path)
		}
		path, err = filepath.Abs(path)
		return m, i, path, err
	}
	return m, 0, "", fmt.Errorf("manifest has no cut %q", id)
}

// EditProject creates a native LosslessCut project beside the original source.
// Existing projects are returned unchanged so reopening never discards edits.
func EditProject(ctx context.Context, manifest, id string) (string, error) {
	m, i, src, err := localCut(manifest, id)
	if err != nil {
		return "", err
	}
	info, err := inspect(ctx, src)
	if err != nil {
		return "", err
	}
	cut := m.Clips[i]
	if cut.End > info.Duration+0.05 {
		return "", fmt.Errorf("cut %s ends beyond source duration %.3fs", id, info.Duration)
	}
	path := filepath.Join(filepath.Dir(src), id+"-proj.llc")
	if existing, err := os.Lstat(path); err == nil {
		if !existing.Mode().IsRegular() {
			return "", fmt.Errorf("project %s is not a regular file", path)
		}
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	project := struct {
		Version       int           `json:"version"`
		MediaFileName string        `json:"mediaFileName"`
		CutSegments   []editSegment `json:"cutSegments"`
	}{Version: 2, MediaFileName: filepath.Base(src), CutSegments: []editSegment{{Start: cut.Start, End: cut.End, Name: id}}}
	b, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	_, writeErr := f.Write(append(b, '\n'))
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return "", errors.Join(err, os.Remove(path))
	}
	return path, nil
}

// Trim imports one retained range from LosslessCut's CSV (seconds) export.
// Only the manifest is replaced; source videos and prepared libraries are untouched.
func Trim(ctx context.Context, manifest, id, csvPath string) error {
	start, end, err := readTrim(csvPath)
	if err != nil {
		return err
	}
	m, i, src, err := localCut(manifest, id)
	if err != nil {
		return err
	}
	info, err := inspect(ctx, src)
	if err != nil {
		return err
	}
	if end > info.Duration+0.05 {
		return fmt.Errorf("trim ends at %.3fs beyond source duration %.3fs", end, info.Duration)
	}
	m.Clips[i].Start, m.Clips[i].End = start, end
	return replaceManifest(manifest, m)
}

func readTrim(path string) (start, end float64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	r := csv.NewReader(io.LimitReader(f, 4<<20))
	r.FieldsPerRecord = -1 // LosslessCut may include a label and tag columns.
	row, err := r.Read()
	if err != nil {
		return 0, 0, fmt.Errorf("read trim CSV (expected one start,end[,label] row in seconds): %w", err)
	}
	if len(row) < 2 {
		return 0, 0, fmt.Errorf("trim CSV requires start,end[,label] in seconds")
	}
	if _, err := r.Read(); err != io.EOF {
		return 0, 0, fmt.Errorf("trim CSV must contain exactly one retained segment; export CSV (seconds)")
	}
	startText := strings.TrimSpace(strings.TrimPrefix(row[0], "\ufeff"))
	if startText != "" {
		start, err = strconv.ParseFloat(startText, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid start time: %w", err)
		}
	}
	end, err = strconv.ParseFloat(strings.TrimSpace(row[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end time (markers cannot be imported): %w", err)
	}
	if start < 0 || math.IsNaN(start) || !library.ValidDuration(end-start) || end-start < 0.5 {
		return 0, 0, fmt.Errorf("trim requires finite times, nonnegative start, and at least 0.5 seconds")
	}
	return start, end, nil
}

func replaceManifest(path string, m Manifest) (err error) {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("manifest must be a regular file (symlinks are refused)")
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".pi-crt-trim-*")
	if err != nil {
		return err
	}
	defer func() {
		if removeErr := os.Remove(f.Name()); !os.IsNotExist(removeErr) {
			err = errors.Join(err, removeErr)
		}
	}()
	_, writeErr := f.Write(append(b, '\n'))
	if err := errors.Join(writeErr, f.Chmod(info.Mode().Perm()), f.Close()); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
