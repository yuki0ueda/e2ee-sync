package credential

import "testing"

func TestBackendsList(t *testing.T) {
	if len(Backends) != 4 {
		t.Errorf("Backends count = %d, want 4", len(Backends))
	}

	expected := []struct {
		name     string
		provider string
	}{
		{"Cloudflare R2", "Cloudflare"},
		{"AWS S3", "AWS"},
		{"Backblaze B2", "B2"},
		{"Other S3-compatible", "Other"},
	}

	for i, exp := range expected {
		if Backends[i].Name != exp.name {
			t.Errorf("Backends[%d].Name = %q, want %q", i, Backends[i].Name, exp.name)
		}
		if Backends[i].Provider != exp.provider {
			t.Errorf("Backends[%d].Provider = %q, want %q", i, Backends[i].Provider, exp.provider)
		}
	}
}
