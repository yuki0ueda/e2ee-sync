package rclone

import "testing"

func TestParseConfigShowOutput(t *testing.T) {
	// Simulate rclone config show output
	tests := []struct {
		name   string
		output string
		want   map[string]string
	}{
		{
			name: "standard S3 remote",
			output: `[cloud-direct]
type = s3
provider = Cloudflare
access_key_id = AKID123456
secret_access_key = mysecret
endpoint = https://abc.r2.cloudflarestorage.com
region = auto
acl = private
`,
			want: map[string]string{
				"type":              "s3",
				"provider":          "Cloudflare",
				"access_key_id":     "AKID123456",
				"secret_access_key": "mysecret",
				"endpoint":          "https://abc.r2.cloudflarestorage.com",
				"region":            "auto",
				"acl":               "private",
			},
		},
		{
			name: "crypt remote with passwords",
			output: `[cloud-crypt]
type = crypt
remote = cloud-direct:e2ee-sync
password = myencpass
password2 = mysalt
filename_encryption = standard
directory_name_encryption = true
`,
			want: map[string]string{
				"type":                      "crypt",
				"remote":                    "cloud-direct:e2ee-sync",
				"password":                  "myencpass",
				"password2":                 "mysalt",
				"filename_encryption":       "standard",
				"directory_name_encryption": "true",
			},
		},
		{
			name: "value with equals sign",
			output: `[test]
key = value=with=equals
`,
			want: map[string]string{
				"key": "value=with=equals",
			},
		},
		{
			name:   "empty output",
			output: "",
			want:   map[string]string{},
		},
		{
			name:   "only section header",
			output: "[remote]\n",
			want:   map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseConfigShowOutput(tt.output)
			if len(got) != len(tt.want) {
				t.Errorf("got %d keys, want %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("key %q = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
