package assets

import "testing"

func TestMatchToken(t *testing.T) {
	tests := []struct {
		pattern string
		query   string
		want    bool
	}{
		{"*", "anything", true},
		{"WALL:*:*:*:*", "WALL:SMOOTH:STONE:GRANITE:--------", true},
		{"WALL:*:TREE_MATERIAL:*:--------", "WALL:SMOOTH:TREE_MATERIAL:OAK:--------", true},
		{"WALL:*:TREE_MATERIAL:*:--------", "WALL:SMOOTH:STONE:OAK:--------", false},
		{"FLOOR:*:*:*:*", "WALL:*:*:*:*", false},
		{"BRANCH:*:*:*:N-S-W-E-", "BRANCH:SMOOTH:TREE:OAK:N-S-W-E-", true},
		{"BRANCH:*:*:*:N-S-W-E-", "BRANCH:SMOOTH:TREE:OAK:--------", false},
	}

	for _, tt := range tests {
		got := matchToken(tt.pattern, tt.query)
		if got != tt.want {
			t.Errorf("matchToken(%q, %q) = %v, want %v", tt.pattern, tt.query, got, tt.want)
		}
	}
}

func TestSpecificityScore(t *testing.T) {
	tests := []struct {
		pattern string
		want    int
	}{
		{"*", 0},
		{"WALL:*:*:*:*", 1},
		{"WALL:*:TREE_MATERIAL:*:*", 2},
		{"WALL:SMOOTH:TREE_MATERIAL:OAK:--------", 5},
	}

	for _, tt := range tests {
		got := specificityScore(tt.pattern)
		if got != tt.want {
			t.Errorf("specificityScore(%q) = %d, want %d", tt.pattern, got, tt.want)
		}
	}
}
