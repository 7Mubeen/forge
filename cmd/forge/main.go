package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"forge/internal/chunk"
	"forge/internal/commit"
	"forge/internal/index"
	"forge/internal/manifest"
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
	case "add":
		err = runAdd(args)
	case "commit":
		err = runCommit(args)
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
	fmt.Println("  add     Add file contents to the index")
	fmt.Println("  commit  Record changes to the repository")
}

func runInit(args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	_, err = repository.Init(absPath)
	if err != nil {
		return err
	}

	fmt.Printf("Initialized empty Forge repository in %s\n", filepath.Join(absPath, repository.ForgeDir))
	return nil
}

func runAdd(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("nothing specified, nothing added")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	repoRoot, err := findRepoRoot(cwd)
	if err != nil {
		return err
	}

	repo, err := repository.Open(repoRoot)
	if err != nil {
		return err
	}

	indexPath := filepath.Join(repo.Root, repository.ForgeDir, "index")
	idx, err := index.Load(indexPath)
	if err != nil {
		return err
	}

	for _, arg := range args {
		absPath, err := filepath.Abs(arg)
		if err != nil {
			return fmt.Errorf("resolving path %s: %w", arg, err)
		}

		if !strings.HasPrefix(absPath, repo.Root) {
			return fmt.Errorf("file %s is outside repository", arg)
		}

		relPath, err := filepath.Rel(repo.Root, absPath)
		if err != nil {
			return fmt.Errorf("calculating relative path: %w", err)
		}

		relPath = filepath.ToSlash(relPath)

		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("stat %s: %w", arg, err)
		}
		if info.IsDir() {
			return fmt.Errorf("adding directories is not yet supported: %s", arg)
		}

		chunker, err := chunk.NewWithSize(repo.ObjectStore, repo.Config.DefaultChunkSize)
		if err != nil {
			return fmt.Errorf("creating chunker: %w", err)
		}

		chunkIDs, err := chunker.ChunkFile(absPath)
		if err != nil {
			return fmt.Errorf("chunking %s: %w", arg, err)
		}

		entry := index.Entry{
			Path:   relPath,
			Size:   info.Size(),
			Chunks: chunkIDs,
		}
		idx.AddOrUpdate(entry)
		fmt.Printf("added: %s\n", relPath)
	}

	if err := idx.Save(indexPath); err != nil {
		return fmt.Errorf("saving index: %w", err)
	}

	return nil
}

func runCommit(args []string) error {
	fs := flag.NewFlagSet("commit", flag.ExitOnError)
	message := fs.String("m", "", "commit message")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *message == "" {
		return fmt.Errorf("commit message is required (use -m \"message\")")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	repoRoot, err := findRepoRoot(cwd)
	if err != nil {
		return err
	}

	repo, err := repository.Open(repoRoot)
	if err != nil {
		return err
	}

	indexPath := filepath.Join(repo.Root, repository.ForgeDir, "index")
	idx, err := index.Load(indexPath)
	if err != nil {
		return err
	}

	if len(idx.Entries) == 0 {
		return fmt.Errorf("nothing to commit (index is empty)")
	}

	// 1. Build the manifest from the index
	m := manifest.New()
	for _, entry := range idx.Entries {
		err := m.AddFile(manifest.File{
			Path:   entry.Path,
			Size:   entry.Size,
			Chunks: entry.Chunks,
		})
		if err != nil {
			return fmt.Errorf("adding file to manifest: %w", err)
		}
	}

	// 2. Store the manifest
	manifestID, err := repo.ManifestStore.Put(m)
	if err != nil {
		return fmt.Errorf("storing manifest: %w", err)
	}

	// 3. Get the parent commit (if any)
	parentID, err := repo.GetHead()
	if err != nil {
		return fmt.Errorf("getting HEAD: %w", err)
	}

	// 4. Create the commit
	var c *commit.Commit
	author := "Forge User <user@local>" // V1 default

	if parentID == "" {
		c, err = commit.New(manifestID, author, *message)
	} else {
		c, err = commit.NewWithParent(parentID, manifestID, author, *message)
	}
	if err != nil {
		return fmt.Errorf("creating commit: %w", err)
	}

	// 5. Store the commit
	commitID, err := repo.CommitStore.Put(c)
	if err != nil {
		return fmt.Errorf("storing commit: %w", err)
	}

	// 6. Update HEAD
	if err := repo.SetHead(commitID); err != nil {
		return fmt.Errorf("updating HEAD: %w", err)
	}

	branchName := "main"
	if parentID == "" {
		fmt.Printf("[%s (root-commit) %s] %s\n", branchName, commitID[:16], *message)
	} else {
		fmt.Printf("[%s %s] %s\n", branchName, commitID[:16], *message)
	}
	fmt.Printf(" %d file(s) staged and committed\n", len(idx.Entries))

	return nil
}

func findRepoRoot(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, repository.ForgeDir)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a forge repository (or any parent)")
		}
		dir = parent
	}
}
