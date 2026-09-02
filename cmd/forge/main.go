package main

import (
	"fmt"
	"os"
	"path/filepath"

	"forge/internal/repository"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = runInit(args)
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "forge: '%s' is not a forge command. See 'forge help'\n", cmd)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("usage: forge <command> [arguments]")
	fmt.Println()
	fmt.Println("These are common Forge commands used in various situations:")
	fmt.Println()
	fmt.Println("start a working area:")
	fmt.Println("  init    Create an empty Forge repository")
	fmt.Println()
	fmt.Println("work on current change:")
	fmt.Println("  (more commands coming soon)")
}

func runInit(args []string) error {
	// Default to the current directory if no path is provided
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Resolve to an absolute path to avoid issues with relative paths later
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Initialize the repository using our internal package
	_, err = repository.Init(absPath)
	if err != nil {
		return err
	}

	fmt.Printf("Initialized empty Forge repository in %s\n", filepath.Join(absPath, repository.ForgeDir))
	return nil
}
