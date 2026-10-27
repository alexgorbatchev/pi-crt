// Package library defines the portable, prepared media library.
package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
)

type Clip struct {
	ID       string  `json:"id"`
	Group    string  `json:"group"`
	File     string  `json:"file"`
	Duration float64 `json:"duration"`
}

type Library struct {
	Standard string `json:"standard"`
	Clips    []Clip `json:"clips"`
}

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}$`)

func ValidID(id string) bool       { return identifier.MatchString(id) }
func ValidDuration(n float64) bool { return n > 0 && !math.IsNaN(n) && !math.IsInf(n, 0) }

func ReadJSON(path string, value any) (err error) {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	d := json.NewDecoder(io.LimitReader(f, 4<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("%s: expected one JSON object", path)
	}
	return nil
}

func Load(dir string) (lib Library, err error) {
	if err := ReadJSON(filepath.Join(dir, "library.json"), &lib); err != nil {
		return lib, err
	}
	if lib.Standard != "ntsc" && lib.Standard != "pal" {
		return lib, fmt.Errorf("unknown standard %q", lib.Standard)
	}
	if len(lib.Clips) == 0 {
		return lib, fmt.Errorf("library contains no clips")
	}
	seen := make(map[string]bool)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return lib, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	for _, c := range lib.Clips {
		if !ValidID(c.ID) || seen[c.ID] || !ValidDuration(c.Duration) || c.Group == "" || !filepath.IsLocal(c.File) {
			return lib, fmt.Errorf("invalid or duplicate clip %q", c.ID)
		}
		seen[c.ID] = true
		info, err := root.Stat(c.File)
		if err != nil {
			return lib, fmt.Errorf("clip %s: %w", c.ID, err)
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return lib, fmt.Errorf("clip %s is not a nonempty regular file", c.ID)
		}
	}
	return lib, nil
}
