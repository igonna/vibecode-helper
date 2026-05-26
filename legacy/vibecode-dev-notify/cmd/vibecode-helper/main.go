package main

import (
	"flag"
	"fmt"
	"os"

	"vibecode-dev-notify/internal/helper"
	"vibecode-dev-notify/internal/runner"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "serve":
		flags := flag.NewFlagSet("serve", flag.ExitOnError)
		addr := flags.String("addr", "127.0.0.1:8766", "HTTP listen address")
		_ = flags.Parse(os.Args[2:])

		if err := helper.Serve(*addr); err != nil {
			fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
			os.Exit(1)
		}
	case "runner":
		flags := flag.NewFlagSet("runner", flag.ExitOnError)
		appID := flags.String("app", "", "app id")
		helperURL := flags.String("helper-url", "http://localhost:8766", "helper URL")
		baseDir := flags.String("base-dir", "", "directory where the runner was launched")
		_ = flags.Parse(os.Args[2:])

		if *appID == "" {
			fmt.Fprintln(os.Stderr, "runner requires --app")
			os.Exit(2)
		}

		if err := runner.Run(*helperURL, *appID, *baseDir); err != nil {
			fmt.Fprintf(os.Stderr, "runner failed: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  vibecode-helper serve [--addr 127.0.0.1:8766]")
	fmt.Fprintln(os.Stderr, "  vibecode-helper runner --app <id> [--helper-url http://localhost:8766] [--base-dir /path]")
}
