package version

import "fmt"

var (
	Version = "dev"
	Commit  = "unknown"
)

func String() string {
	v := Version
	if len(v) > 0 && v[0] != 'v' {
		v = "v" + v
	}
	return fmt.Sprintf("%s (%s)", v, Commit)
}
