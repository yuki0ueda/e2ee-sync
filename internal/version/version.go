package version

import "fmt"

var (
	Version = "dev"
	Commit  = "unknown"
)

func String() string {
	return fmt.Sprintf("v%s (%s)", Version, Commit)
}
