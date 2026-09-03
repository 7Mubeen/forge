package main

import (
	"flag"
	"fmt"
	"io"
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
	case "checkout":
		err = runCheckout(args)
	case "fsck":
		err = runFsck(args)
	case "gc":
		err = runGc(args)
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
	fmt.Println("  init      Create an empty Forge repository")
	fmt.Println()
	fmt.Println("work on current change:")
	fmt.Println("  add       Add file contents to the index")
	fmt.Println("  commit    Record changes to the repository")
	fmt.Println()
	fmt.Println("examine the history and state:")
	fmt.Println("  log       Show commit logs")
	fmt.Println("  status    Show the working tree status")
	fmt.Println()
	fmt.Println("switch branches or restore working tree files:")
	fmt.Println("  checkout  Switch commits and restore files")
	fmt.Println()
	fmt.Println("maintenance and integrity:")
	fmt.Println("  fsck      Verify the integrity of the repository")
	fmt.Println("  gc        Garbage-collect unreachable objects")
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

		hashFile, err := os.Open(absPath)
		if err != nil {
			return fmt.Errorf("opening %s for hashing: %w", arg, err)
		}
		overallHash, err := hash.SumReader(hashFile)
		hashFile.Close()
		if err != nil {
			return fmt.Errorf("hashing %s: %w", arg, err)
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

	indexPath := filepath.Join(repo.Root, repository.ForgeDir, "index")
	idx, err := index.Load(indexPath)
	if err != nil {
		return fmt.Errorf("loading index: %w", err)
	}

	workDirFiles := make(map[string]string)
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

	headFiles := make(map[string]manifest.File)
	for _, f := range headManifest.Files {
		headFiles[f.Path] = f
	}

	indexFiles := make(map[string]index.Entry)
	for _, e := range idx.Entries {
		indexFiles[e.Path] = e
	}

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

func runCheckout(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("you must specify a commit ID to checkout")
	}

	commitID := args[0]

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

	c, err := repo.CommitStore.Get(commitID)
	if err != nil {
		return fmt.Errorf("loading commit %s: %w", commitID, err)
	}

	m, err := repo.ManifestStore.Get(c.Manifest)
	if err != nil {
		return fmt.Errorf("loading manifest for commit %s: %w", commitID, err)
	}

	indexPath := filepath.Join(repo.Root, repository.ForgeDir, "index")

	oldIdx, err := index.Load(indexPath)
	if err != nil {
		return fmt.Errorf("loading old index: %w", err)
	}

	newManifestPaths := make(map[string]bool)
	for _, file := range m.Files {
		newManifestPaths[file.Path] = true
	}

	for _, entry := range oldIdx.Entries {
		if !newManifestPaths[entry.Path] {
			absPath := filepath.Join(repo.Root, entry.Path)
			if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing old file %s: %w", entry.Path, err)
			}
			dir := filepath.Dir(absPath)
			for dir != repoRoot {
				if err := os.Remove(dir); err != nil {
					break
				}
				dir = filepath.Dir(dir)
			}
		}
	}

	for _, file := range m.Files {
		absPath := filepath.Join(repo.Root, file.Path)

		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", file.Path, err)
		}

		outFile, err := os.Create(absPath)
		if err != nil {
			return fmt.Errorf("creating file %s: %w", file.Path, err)
		}

		for _, chunkID := range file.Chunks {
			reader, err := repo.ObjectStore.Get(chunkID)
			if err != nil {
				outFile.Close()
				return fmt.Errorf("reading chunk %s for %s: %w", chunkID, file.Path, err)
			}

			_, err = io.Copy(outFile, reader)
			reader.Close()
			if err != nil {
				outFile.Close()
				return fmt.Errorf("writing chunk %s for %s: %w", chunkID, file.Path, err)
			}
		}
		outFile.Close()
	}

	newIdx := &index.Index{}

	for _, file := range m.Files {
		absPath := filepath.Join(repo.Root, file.Path)

		hashFile, err := os.Open(absPath)
		if err != nil {
			return fmt.Errorf("opening %s for hashing: %w", file.Path, err)
		}
		overallHash, err := hash.SumReader(hashFile)
		hashFile.Close()
		if err != nil {
			return fmt.Errorf("hashing %s: %w", file.Path, err)
		}

		newIdx.AddOrUpdate(index.Entry{
			Path:   file.Path,
			Size:   file.Size,
			Hash:   overallHash,
			Chunks: file.Chunks,
		})
	}

	if err := newIdx.Save(indexPath); err != nil {
		return fmt.Errorf("saving index: %w", err)
	}

	if err := repo.SetHead(commitID); err != nil {
		return fmt.Errorf("updating HEAD: %w", err)
	}

	fmt.Printf("Switched to commit %s\n", commitID)
	return nil
}

func runFsck(args []string) error {
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
		fmt.Println("no commits to verify")
		return nil
	}

	var commitsChecked, manifestsChecked, chunksChecked int
	var errorsFound int

	currentID := headID
	for currentID != "" {
		if err := repo.CommitStore.Verify(currentID); err != nil {
			fmt.Printf("corrupt commit: %s (%v)\n", currentID, err)
			errorsFound++
			break
		}
		commitsChecked++

		c, err := repo.CommitStore.Get(currentID)
		if err != nil {
			fmt.Printf("error reading commit %s: %v\n", currentID, err)
			errorsFound++
			break
		}

		if err := repo.ManifestStore.Verify(c.Manifest); err != nil {
			fmt.Printf("corrupt manifest: %s (%v)\n", c.Manifest, err)
			errorsFound++
		} else {
			manifestsChecked++
			m, err := repo.ManifestStore.Get(c.Manifest)
			if err != nil {
				fmt.Printf("error reading manifest %s: %v\n", c.Manifest, err)
				errorsFound++
			} else {
				for _, f := range m.Files {
					for _, chunkID := range f.Chunks {
						if err := repo.ObjectStore.Verify(chunkID); err != nil {
							fmt.Printf("corrupt chunk: %s (%v)\n", chunkID, err)
							errorsFound++
						} else {
							chunksChecked++
						}
					}
				}
			}
		}

		currentID = c.Parent
	}

	fmt.Printf("\nfsck summary:\n")
	fmt.Printf("  commits checked:   %d\n", commitsChecked)
	fmt.Printf("  manifests checked: %d\n", manifestsChecked)
	fmt.Printf("  chunks checked:    %d\n", chunksChecked)
	fmt.Printf("  errors found:      %d\n", errorsFound)

	if errorsFound > 0 {
		return fmt.Errorf("repository has %d errors", errorsFound)
	}
	fmt.Println("repository is healthy")
	return nil
}

func runGc(args []string) error {
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

	reachable := make(map[string]bool)

	headID, err := repo.GetHead()
	if err != nil {
		return fmt.Errorf("getting HEAD: %w", err)
	}

	if headID != "" {
		currentID := headID
		for currentID != "" {
			reachable[currentID] = true
			c, err := repo.CommitStore.Get(currentID)
			if err != nil {
				return fmt.Errorf("reading commit %s: %w", currentID, err)
			}
			reachable[c.Manifest] = true
			m, err := repo.ManifestStore.Get(c.Manifest)
			if err != nil {
				return fmt.Errorf("reading manifest %s: %w", c.Manifest, err)
			}
			for _, f := range m.Files {
				for _, chunkID := range f.Chunks {
					reachable[chunkID] = true
				}
			}
			currentID = c.Parent
		}
	}

	objectsDir := filepath.Join(repo.Root, repository.ForgeDir, repository.ObjectsDir)
	var removed int
	var kept int

	err = filepath.Walk(objectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(objectsDir, path)
		if err != nil {
			return nil
		}

		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) != 2 {
			return nil
		}

		id := "blake3:" + parts[0] + parts[1]

		if !reachable[id] {
			if err := os.Remove(path); err == nil {
				removed++
			}
		} else {
			kept++
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("walking object store: %w", err)
	}

	fmt.Printf("gc summary:\n")
	fmt.Printf("  reachable objects: %d\n", len(reachable))
	fmt.Printf("  objects kept:      %d\n", kept)
	fmt.Printf("  objects removed:   %d\n", removed)

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
