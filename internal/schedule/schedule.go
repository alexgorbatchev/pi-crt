// Package schedule chooses clips without replacement and preserves recency across rounds.
package schedule

import (
	"fmt"
	"math/rand/v2"
	"pi-crt/internal/library"
)

type Bag struct {
	clips     []library.Clip
	remaining []int
	recent    []int
	window    int
	lastGroup string
	excluded  map[int]bool
}

// New clamps the recent window to keep even very small libraries playable.
func New(clips []library.Clip, recent int) (*Bag, error) {
	if len(clips) == 0 || recent < 0 {
		return nil, fmt.Errorf("need clips and a nonnegative recent window")
	}
	return &Bag{clips: append([]library.Clip(nil), clips...), window: min(recent, len(clips)-1), excluded: make(map[int]bool)}, nil
}

func (b *Bag) Next() library.Clip {
	if len(b.remaining) == 0 {
		for i := range b.clips {
			if !b.excluded[i] {
				b.remaining = append(b.remaining, i)
			}
		}
	}
	var candidates, different []int
	for pos, i := range b.remaining {
		blocked := false
		for _, r := range b.recent {
			if i == r {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		candidates = append(candidates, pos)
		if b.clips[i].Group != b.lastGroup {
			different = append(different, pos)
		}
	}
	if len(different) > 0 {
		candidates = different
	}
	pos := candidates[rand.IntN(len(candidates))]
	i := b.remaining[pos]
	b.remaining = append(b.remaining[:pos], b.remaining[pos+1:]...)
	b.recent = append(b.recent, i)
	if len(b.recent) > b.window {
		b.recent = b.recent[1:]
	}
	b.lastGroup = b.clips[i].Group
	return b.clips[i]
}

// Exclude removes a failed clip without forgetting the history of playable clips.
func (b *Bag) Exclude(id string) error {
	index := -1
	for i, c := range b.clips {
		if c.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("unknown clip %q", id)
	}
	if b.excluded[index] {
		return nil
	}
	if len(b.excluded)+1 == len(b.clips) {
		return fmt.Errorf("cannot exclude the last clip")
	}
	b.excluded[index] = true
	remaining := b.remaining[:0]
	for _, i := range b.remaining {
		if i != index {
			remaining = append(remaining, i)
		}
	}
	b.remaining = remaining
	recent := b.recent[:0]
	for _, i := range b.recent {
		if i != index {
			recent = append(recent, i)
		}
	}
	b.window = min(b.window, len(b.clips)-len(b.excluded)-1)
	if len(recent) > b.window {
		recent = recent[len(recent)-b.window:]
	}
	b.recent = recent
	return nil
}
