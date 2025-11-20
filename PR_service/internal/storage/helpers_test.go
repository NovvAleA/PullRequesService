package storage_test

import (
	"testing"

	"github.com/example/pr-reviewer/internal/storage"
)

func TestPickRandomDistinct(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		n    int
	}{
		{"take 2", []string{"u1", "u2", "u3"}, 2},
		{"take 1", []string{"u1", "u2"}, 1},
		{"take all", []string{"u1"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := storage.PickForTest(tt.in, tt.n) // небольшой wrapper
			if len(out) > tt.n {
				t.Fatalf("got %d want <= %d", len(out), tt.n)
			}
		})
	}
}
