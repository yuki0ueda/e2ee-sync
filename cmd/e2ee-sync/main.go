package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/yuki0ueda/e2ee-sync/internal/credential"
	"github.com/yuki0ueda/e2ee-sync/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		interactiveMenu()
		return
	}
	switch os.Args[1] {
	case "setup":
		runSetup()
	case "share":
		runShare()
	case "join":
		runJoin()
	case "daemon":
		runDaemon()
	case "upgrade":
		runUpgrade()
	case "verify":
		runVerify()
	case "uninstall":
		runUninstall()
	case "version", "--version":
		fmt.Printf("e2ee-sync %s\n", version.String())
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `e2ee-sync %s

Usage:
  e2ee-sync <command>

Commands:
  setup       Full device setup (interactive)
  share       Share config to a new device (run on existing device)
  join        Join using config from another device
  daemon      Run sync daemon (usually started by OS)
  upgrade     Update binary in place
  verify      Verify existing configuration
  uninstall   Remove daemon and configuration
  version     Show version
`, version.String())
}

func interactiveMenu() {
	fmt.Printf("\n=== E2EE File Sync %s ===\n\n", version.String())
	fmt.Println("  1) Setup    — New device setup")
	fmt.Println("  2) Verify   — Verify connection and configuration")
	fmt.Println("  3) Quit")
	fmt.Println()

	choice, err := credential.ReadLine("Select [1-3]: ")
	if err != nil {
		fatalf("Failed to read input: %v", err)
	}

	switch strings.TrimSpace(choice) {
	case "1":
		runSetup()
	case "2":
		runVerify()
	case "3":
		return
	default:
		fmt.Fprintln(os.Stderr, "Invalid selection.")
		os.Exit(1)
	}

	waitIfDoubleClicked()
}
