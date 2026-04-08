package template

import (
	"bytes"
	"text/template"
)

// RcloneConfData holds obscured credentials for rclone.conf generation.
type RcloneConfData struct {
	UseHub              bool
	WebDAVPassObscured  string
	EncPasswordObscured string
	EncSaltObscured     string
	R2AccessKey         string
	R2SecretKeyObscured string
	R2AccountID         string
}

const rcloneConfTmpl = `{{- if .UseHub -}}
[hub-webdav]
type = webdav
url = http://e2ee-sync-hub:8080
vendor = other
user = rclone
pass = {{.WebDAVPassObscured}}

[hub-crypt]
type = crypt
remote = hub-webdav:
password = {{.EncPasswordObscured}}
password2 = {{.EncSaltObscured}}
filename_encryption = standard
directory_name_encryption = true

{{end -}}
[r2-direct]
type = s3
provider = Cloudflare
access_key_id = {{.R2AccessKey}}
secret_access_key = {{.R2SecretKeyObscured}}
endpoint = https://{{.R2AccountID}}.r2.cloudflarestorage.com
region = auto
acl = private

[r2-crypt]
type = crypt
remote = r2-direct:e2ee-sync
password = {{.EncPasswordObscured}}
password2 = {{.EncSaltObscured}}
filename_encryption = standard
directory_name_encryption = true
`

// RenderRcloneConf generates rclone.conf content from the given data.
func RenderRcloneConf(data RcloneConfData) (string, error) {
	tmpl, err := template.New("rclone.conf").Parse(rcloneConfTmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// AutosyncConfigData holds configuration for the autosync config file.
type AutosyncConfigData struct {
	UseHub         bool
	SyncDir        string
	FilterFilePath string
}

const autosyncConfigTmpl = `sync_dir: {{.SyncDir}}
{{- if .UseHub}}
primary_remote: hub-crypt:
fallback_remote: r2-crypt:
{{- else}}
primary_remote: r2-crypt:
{{- end}}
rclone_path: rclone
filter_file: {{.FilterFilePath}}
debounce_sec: 5
poll_interval_sec: 300
`

// RenderAutosyncConfig generates the autosync config.json content.
func RenderAutosyncConfig(data AutosyncConfigData) (string, error) {
	tmpl, err := template.New("config").Parse(autosyncConfigTmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// FilterRules returns the content for filter-rules.txt.
func FilterRules() string {
	return `# E2EE Sync filter rules
# Exclude OS-generated files
- .DS_Store
- Thumbs.db
- desktop.ini
- .Spotlight-V100/**
- .Trashes/**
- .fseventsd/**
# Exclude temporary files
- ~*
- *.tmp
- *.swp
- *.swo
# Exclude rclone internal files
- .rclone-test
`
}
