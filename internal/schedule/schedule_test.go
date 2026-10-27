package schedule

import (
	"fmt"
	"pi-crt/internal/library"
	"testing"
)

func TestNoRepeatsAcrossRounds(t *testing.T) {
	for n := 1; n <= 30; n++ {
		for window := 0; window <= n+2; window++ {
			clips := make([]library.Clip, n)
			for i := range clips {
				clips[i] = library.Clip{ID: fmt.Sprint(i), Group: fmt.Sprint(i % 4)}
			}
			bag, err := New(clips, window)
			if err != nil {
				t.Fatal(err)
			}
			last := make(map[string]int)
			for round := range 20 {
				seen := make(map[string]bool)
				for step := range n {
					c := bag.Next()
					if seen[c.ID] {
						t.Fatalf("duplicate within round: %s", c.ID)
					}
					seen[c.ID] = true
					pos := round*n + step
					if prev, ok := last[c.ID]; ok && pos-prev <= min(window, n-1) {
						t.Fatalf("n=%d window=%d repeat distance=%d", n, window, pos-prev)
					}
					last[c.ID] = pos
				}
			}
		}
	}
}

func TestGroupPreferenceAndInvalidInput(t *testing.T) {
	if _, err := New(nil, 1); err == nil {
		t.Fatal("accepted empty library")
	}
	if _, err := New([]library.Clip{{ID: "a"}}, -1); err == nil {
		t.Fatal("accepted negative window")
	}
	b, _ := New([]library.Clip{{ID: "a", Group: "A"}, {ID: "b", Group: "A"}, {ID: "c", Group: "B"}, {ID: "d", Group: "B"}}, 0)
	for range 100 {
		// Whenever an eligible different group exists, it must win.
		previous := b.lastGroup
		available := len(b.remaining) == 0
		for _, i := range b.remaining {
			available = available || b.clips[i].Group != previous
		}
		c := b.Next()
		if available && c.Group == previous {
			t.Fatal("missed different group")
		}
	}
}

func TestExcludePreservesLiveHistory(t *testing.T) {
	b, _ := New([]library.Clip{{ID: "a", Group: "A"}, {ID: "b", Group: "B"}, {ID: "c", Group: "C"}, {ID: "d", Group: "D"}}, 2)
	first := b.Next()
	second := b.Next()
	if err := b.Exclude(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.Exclude(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.Exclude("unknown"); err == nil {
		t.Fatal("accepted unknown clip")
	}
	last := second.ID
	for range 100 {
		c := b.Next()
		if c.ID == first.ID || c.ID == last {
			t.Fatal("forgot exclusions or history")
		}
		last = c.ID
	}
	for _, c := range b.clips {
		if c.ID != first.ID {
			_ = b.Exclude(c.ID)
		}
	}
	if err := b.Exclude(b.Next().ID); err == nil {
		t.Fatal("removed last playable clip")
	}
}
