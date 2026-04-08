package main

import "testing"

func TestIsIgnored(t *testing.T) {
	tests := []struct {
		name    string
		ignored bool
	}{
		{".DS_Store", true},
		{"Thumbs.db", true},
		{"desktop.ini", true},
		{"~tempfile", true},
		{"file.tmp", true},
		{"file.swp", true},
		{"file.swo", true},
		{"file.partial", true},
		{"document.pdf", false},
		{"photo.jpg", false},
		{"README.md", false},
		{".gitignore", false},
		{"notes.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIgnored(tt.name); got != tt.ignored {
				t.Errorf("isIgnored(%q) = %v, want %v", tt.name, got, tt.ignored)
			}
		})
	}
}
