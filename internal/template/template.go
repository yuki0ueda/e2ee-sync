package template

import (
	"bytes"
	"text/template"
)

// AutosyncConfigData holds configuration for the autosync config file.
type AutosyncConfigData struct {
	UseHub         bool
	SyncDir        string
	TrashDir       string
	FilterFilePath string
}

const autosyncConfigTmpl = `sync_dir: {{.SyncDir}}
trash_dir: {{.TrashDir}}
{{- if .UseHub}}
primary_remote: hub-crypt:
fallback_remote: cloud-crypt:
{{- else}}
primary_remote: cloud-crypt:
{{- end}}
rclone_path: rclone
filter_file: {{.FilterFilePath}}
debounce_sec: 5
poll_interval_sec: 300
trash_retain_days: 30
hub_timeout_sec: 5
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
# Exclude e2ee-sync trash (backup of deleted/overwritten files)
- .trash/**
`
}
