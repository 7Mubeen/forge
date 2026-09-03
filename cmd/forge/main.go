package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"forge/internal/chunk"
	"forge/internal/commit"
	"forge/internal/hash"
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
	case "log":
		err = runLog(args)
	case "status":
		err = runStatus(args)
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
	fmt.Println()
	fmt.Println("examine the history and state:")
	fmt.Println("  log     Show commit logs")
	fmt.Println("  status  Show the working tree status")
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

		// 1. Calculate the overall hash for fast status checks later
		hashFile, err := os.Open(absPath)
		if err != nil {
			return fmt.Errorf("opening %s for hashing: %w", arg, err)
		}
		overallHash, err := hash.SumReader(hashFile)
		hashFile.Close()
		if err != nil {
			return fmt.Errorf("hashing %s: %w", arg, err)
		}

		// 2. Chunk the file for content-addressed storage
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
			Hash:   overallHash,
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

	manifestID, err := repo.ManifestStore.Put(m)
	if err != nil {
		return fmt.Errorf("storing manifest: %w", err)
	}

	parentID, err := repo.GetHead()
	if err != nil {
		return fmt.Errorf("getting HEAD: %w", err)
	}

	var c *commit.Commit
	author := "Forge User <user@local>"

	if parentID == "" {
		c, err = commit.New(manifestID, author, *message)
	} else {
		c, err = commit.NewWithParent(parentID, manifestID, author, *message)
	}
	if err != nil {
		return fmt.Errorf("creating commit: %w", err)
	}

	commitID, err := repo.CommitStore.Put(c)
	if err != nil {
		return fmt.Errorf("storing commit: %w", err)
	}

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

func runLog(args []string) error {
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

	headID, err := repo.GetHead()
	if err != nil {
		return fmt.Errorf("getting HEAD: %w", err)
	}

	if headID == "" {
		fmt.Println("fatal: your current branch 'main' does not have any commits yet")
		return nil
	}

	currentID := headID
	for currentID != "" {
		c, err := repo.CommitStore.Get(currentID)
		if err != nil {
			return fmt.Errorf("reading commit %s: %w", currentID, err)
		}

		fmt.Printf("commit %s\n", currentID)
		fmt.Printf("Author: %s\n", c.Author)
		fmt.Printf("Date:   %s\n", c.Timestamp.Format("2006-01-02 15:04:05 -0700"))
		fmt.Printf("\n    %s\n\n", c.Message)

		currentID = c.Parent
	}

	return nil
}

func runStatus(args []string) error {
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

	// 1. Get HEAD manifest
	headID, err := repo.GetHead()
	if err != nil {
		return fmt.Errorf("getting HEAD: %w", err)
	}

	var headManifest *manifest.Manifest
	if headID != "" {
		headCommit, err := repo.CommitStore.Get(headID)
		if err != nil {
			return fmt.Errorf("reading HEAD commit: %w", err)
		}
		headManifest, err = repo.ManifestStore.Get(headCommit.Manifest)
		if err != nil {
			return fmt.Errorf("reading HEAD manifest: %w", err)
		}
	} else {
		headManifest = manifest.New()
	}

	// 2. Get current index
	indexPath := filepath.Join(repo.Root, repository.ForgeDir, "index")
	idx, err := index.Load(indexPath)
	if err != nil {
		return fmt.Errorf("loading index: %w", err)
	}

	// 3. Get working directory files
	workDirFiles := make(map[string]string) // relPath -> absPath
	err = filepath.Walk(repo.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == repository.ForgeDir {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(repo.Root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		workDirFiles[rel] = path
		return nil
	})
	if err != nil {
		return fmt.Errorf("walking working directory: %w", err)
	}

	// Maps for easy lookup
	headFiles := make(map[string]manifest.File)
	for _, f := range headManifest.Files {
		headFiles[f.Path] = f
	}

	indexFiles := make(map[string]index.Entry)
	for _, e := range idx.Entries {
		indexFiles[e.Path] = e
	}

	// 4. Compare HEAD vs Index (Staged changes)
	var stagedNew, stagedModified, stagedDeleted []string

	for path, idxEntry := range indexFiles {
		headFile, inHead := headFiles[path]
		if !inHead {
			stagedNew = append(stagedNew, path)
		} else {
			if !stringSlicesEqual(idxEntry.Chunks, headFile.Chunks) {
				stagedModified = append(stagedModified, path)
			}
		}
	}
	for path := range headFiles {
		if _, inIndex := indexFiles[path]; !inIndex {
			stagedDeleted = append(stagedDeleted, path)
		}
	}

	// 5. Compare Index vs Working Dir (Unstaged changes)
	var unstagedModified, unstagedDeleted []string
	var untracked []string

	for path, idxEntry := range indexFiles {
		absPath, inWorkDir := workDirFiles[path]
		if !inWorkDir {
			unstagedDeleted = append(unstagedDeleted, path)
		} else {
			modified, err := isModified(absPath, idxEntry.Hash)
			if err != nil {
				return fmt.Errorf("checking modification for %s: %w", path, err)
			}
			if modified {
				unstagedModified = append(unstagedModified, path)
			}
		}
	}

	for path := range workDirFiles {
		if _, inIndex := indexFiles[path]; !inIndex {
			untracked = append(untracked, path)
		}
	}

	// 6. Print output
	fmt.Println("On branch main")

	hasStaged := len(stagedNew) > 0 || len(stagedModified) > 0 || len(stagedDeleted) > 0
	if hasStaged {
		fmt.Println("\nChanges to be committed:")
		printStatusList("  new file:   ", stagedNew)
		printStatusList("  modified:   ", stagedModified)
		printStatusList("  deleted:    ", stagedDeleted)
	}

	hasUnstaged := len(unstagedModified) > 0 || len(unstagedDeleted) > 0
	if hasUnstaged {
		fmt.Println("\nChanges not staged for commit:")
		printStatusList("  modified:   ", unstagedModified)
		printStatusList("  deleted:    ", unstagedDeleted)
	}

	if len(untracked) > 0 {
		fmt.Println("\nUntracked files:")
		printStatusList("  ", untracked)
	}

	if !hasStaged && !hasUnstaged && len(untracked) == 0 {
		fmt.Println("\nnothing to commit, working tree clean")
	}

	return nil
}

func isModified(absPath string, expectedHash string) (bool, error) {
	file, err := os.Open(absPath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	actualHash, err := hash.SumReader(file)
	if err != nil {
		return false, err
	}

	return actualHash != expectedHash, nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func printStatusList(prefix string, items []string) {
	if len(items) == 0 {
		return
	}
	sort.Strings(items)
	for _, item := range items {
		fmt.Printf("%s%s\n", prefix, item)
	}
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
