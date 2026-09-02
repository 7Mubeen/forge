# Forge Development Log

This document records the development of Forge, including important implementation steps, experiments, architectural decisions, and lessons learned.

---

## 2026-08-29 — Project Foundation

### Project initialized

Created the Forge Git repository and initialized the Go module.

Technology:

* Language: Go
* Version: Go 1.27
* License: Apache License 2.0
* Hashing library: BLAKE3

### Initial project files

```text
forge/
├── .gitignore
├── HANDBOOK.md
├── LICENSE
├── README.md
├── go.mod
└── go.sum
```

### Project documentation

Created `HANDBOOK.md` defining:

* Forge's long-term vision
* The problem Forge is intended to solve
* Core principles
* Content-addressed storage
* Chunking
* Deduplication
* Manifests
* Commits
* Integrity verification
* V1 scope
* Long-term networking/P2P goals

### V1 scope

Forge V1 is intentionally local and offline.

V1 will focus on:

1. Content-addressed objects
2. Cryptographic hashing
3. File chunking
4. Deduplication
5. Manifests
6. Commits
7. Version history
8. Checkout
9. Status
10. Integrity verification
11. Garbage collection

Networking and distributed functionality are explicitly postponed.

### Architectural principle

The project follows this progression:

```text
Hashing
   ↓
Object storage
   ↓
Chunking
   ↓
Manifest
   ↓
Commit
   ↓
Checkout
```

Only after the local foundation is correct will Forge introduce:

```text
Remote storage
   ↓
Partial fetching
   ↓
Peer-to-peer distribution
```

### First commit

```text
b930672 docs: define Forge project vision and V1
```

This commit establishes the project's documentation and initial configuration.

---

## Next Task

### Content-addressed hashing

The next implementation task is to build Forge's hashing primitive.

Goals:

* Define the Forge object ID format
* Implement BLAKE3 hashing
* Support streaming file hashing
* Create deterministic hashes
* Add unit tests
* Keep the hashing API independent of the CLI

The next expected commit:

```text
feat: implement content hashing
```


## 2026-08-29 — Content Hashing

### Implemented

Created the first Forge implementation package:

```text
internal/hash/
├── hash.go
└── hash_test.go
```

The package provides two operations:

```text
Sum([]byte)
SumReader(io.Reader)
```

Both produce a Forge content ID using BLAKE3.

### Object ID format

Forge V1 currently represents content IDs as:

```text
blake3:<64-character lowercase hexadecimal digest>
```

The algorithm identifier is included in the ID so that future versions can support additional hashing algorithms without making existing IDs ambiguous.

### Streaming

`SumReader` uses `io.Reader` and `io.Copy`, allowing Forge to hash large data streams without loading the entire input into memory.

This is an important requirement because Forge is intended to handle very large datasets.

### Tests

The following behaviors are covered:

* Deterministic hashing
* Different data produces different hashes
* Object ID format validation
* Reader hashing produces the same result as byte hashing
* Empty input handling

All tests currently pass:

```text
go test ./...

ok    forge/internal/hash
```

### Next Task

Build the **content-addressed object store**.

The object store will take content, calculate its Forge object ID, and persist the content under that identity.

Conceptually:

```text
content
   ↓
hash
   ↓
object ID
   ↓
object store
```

The next implementation should establish:

* Object storage layout
* Atomic object writes
* Object existence checks
* Object retrieval
* Object integrity verification
* Deduplication behavior

2026-08-31 — Content-Addressed Object Store
Implemented

Built the first Forge content-addressed object store:

internal/object/
├── store.go
└── store_test.go


The object store provides:

Put
Get
Exists
Verify

Object storage

Objects are stored beneath:

.forge/objects/


using a two-character hexadecimal fan-out:

objects/
├── ab/
│   └── cdef...
├── 42/
│   └── 9876...
└── ...


The logical object ID remains:

blake3:<64-character lowercase hexadecimal digest>

Streaming writes

Put accepts an io.Reader.

Content is streamed into a temporary file while BLAKE3 calculates the object ID.

The complete object is therefore never required to exist in memory.

Immutability

Objects are treated as immutable.

The final object is installed using create-if-absent semantics so an existing object cannot be overwritten.

Deduplication

If identical content is added multiple times, it produces the same object ID and is stored only once.

Integrity verification

Verify streams the stored object through the hashing layer and compares the calculated ID with the requested object ID.

Corrupted objects are detected.

Testing

The object store tests cover:

Put and Get
Deduplication
Object existence
Missing objects
Integrity verification
Corruption detection
Empty objects
Large streamed objects
Invalid object IDs
Concurrent duplicate writes

All tests pass:

go test ./...

ok   forge/internal/hash
ok   forge/internal/object


The race detector also passes:

go test -race ./...

ok   forge/internal/hash
ok   forge/internal/object

Architecture

Forge now has the following foundation:

Content
   ↓
BLAKE3 Hash
   ↓
Object ID
   ↓
Content-Addressed Object Store
   ↓
Immutable Object

Next Task

Implement file chunking.

The chunking layer should:

Stream files without loading them into memory
Split large files into chunks
Store chunks through the object store
Return the ordered list of chunk object IDs
Start with fixed-size chunks
Keep the chunking interface replaceable for future content-defined chunking

Expected progression:

File
 │
 ▼
Chunker
 │
 ├── Chunk 1 → Object Store
 ├── Chunk 2 → Object Store
 ├── Chunk 3 → Object Store
 └── ...


Expected next commit:

feat: implement file chunking


---

## 2026-08-31 — File Chunking

### Implemented

Created the Forge chunking package:

```text
internal/chunk/
├── chunker.go
└── chunker_test.go


The chunking layer sits between file input and the content-addressed object store.

Conceptually:

File
  ↓
Chunker
  ↓
Chunks
  ↓
Hash
  ↓
Object Store

V1 Chunking Strategy

Forge V1 currently uses fixed-size chunks.

The default chunk size is:

4 MiB


The chunk size can also be configured through the chunker API.

Content-defined chunking is intentionally postponed. The chunking API is kept independent enough that a future implementation can replace fixed-size chunking without changing the higher-level manifest model.

Streaming

Chunking operates on an io.Reader.

Files are processed incrementally rather than loaded entirely into memory.

For a large file:

10 TB file
    ↓
4 MiB chunk
    ↓
store
    ↓
4 MiB chunk
    ↓
store
    ↓
...


Memory usage therefore remains bounded by the configured chunk size.

Chunk Identity

Every chunk is stored through the existing content-addressed object store.

The chunk ID is therefore the BLAKE3 content ID:

chunk data
    ↓
BLAKE3
    ↓
blake3:<64-character hexadecimal digest>


The chunker returns the ordered list of chunk IDs.

For example:

file
├── chunk A
├── chunk B
└── chunk C


becomes:

[
    "blake3:...",
    "blake3:...",
    "blake3:..."
]


Chunk order is preserved because it is required to reconstruct the original file.

Deduplication

Chunking automatically benefits from the object store's content addressing.

If identical chunks occur multiple times, they produce the same object ID.

For example:

File A:
A B C

File B:
A D C


The object store only needs one copy of:

A
C


This establishes the first practical layer of Forge's large-data deduplication model.

Tests

The chunking package tests:

Fixed-size chunking
Files whose size is exactly divisible by the chunk size
Files smaller than one chunk
Empty files
File-based chunking
Missing files
Invalid chunk sizes
Nil inputs
Chunk ID generation
Chunk ordering
Duplicate chunk detection

Both normal tests and the race detector pass:

go test ./...

ok  	forge/internal/chunk
ok  	forge/internal/hash
ok  	forge/internal/object

go test -race ./...

ok  	forge/internal/chunk
ok  	forge/internal/hash
ok  	forge/internal/object

Architecture After Chunking

Forge's current foundation is:

Hashing
   ↓
Object Storage
   ↓
Chunking


The next layer is the manifest.

Next Task

Implement the manifest layer.

The manifest will describe a filesystem state using:

Repository-relative file paths
File sizes
Ordered chunk IDs
A manifest format version
Deterministic serialization

The manifest should eventually become a content-addressed object itself:

Filesystem
    ↓
Manifest
    ↓
Serialize
    ↓
BLAKE3
    ↓
Manifest Object ID


This will establish the foundation for Forge commits.

The next expected commit will be:

feat: implement manifests

:::

After saving it, verify the file:

```bash
tail -n 100 DEVELOPMENT.md


Then commit the completed chunking layer:

git status
git add DEVELOPMENT.md internal/chunk
git commit -m "feat: implement file chunking"


Verify:

git log --oneline -3


You should now have roughly:

<new> feat: implement file chunking
b930672 docs: define Forge project vision and V1

2. Then create the manifest package

After the commit, create:

mkdir -p internal/manifest
touch internal/manifest/manifest.go
touch internal/manifest/manifest_test.go


Then we'll implement the manifest as a pure data-model package first.

I recommend that we keep the first manifest layer focused on:

Manifest
├── Version
└── Files[]
      ├── Path
      ├── Size
      └── Chunks[]


and not yet connect manifest persistence to the object store.

That separation gives us:

hash       → identity
object     → storage
chunk      → file decomposition
manifest   → filesystem description


Then, once the manifest itself is proven, we'll add the operation that turns its canonical bytes into an object-store ID.

This keeps each layer testable and follows the Handbook's rule:

Do not build the future before proving the present.


---

## 2026-08-31 — Manifest Layer

### Implemented

Created the Forge manifest package:

```text
internal/manifest/
├── manifest.go
└── manifest_test.go


The manifest represents a complete filesystem state without storing file contents directly.

A manifest contains:

Manifest
├── Version
└── Files
    ├── Path
    ├── Size
    └── Chunks


Each file references the ordered content IDs of its chunks.

Deterministic Representation

Manifest files are serialized in deterministic path order.

For example, these insertion orders:

z.txt
a.txt


and:

a.txt
z.txt


produce the same canonical serialized representation.

This is required because the manifest will eventually be content-addressed.

The intended flow is:

Manifest
   ↓
Canonical serialization
   ↓
BLAKE3
   ↓
Manifest object ID

Path Handling

Manifest paths are repository-relative.

Absolute paths are rejected:

/etc/passwd


Parent traversal outside the repository is rejected:

../secret


Paths are normalized using forward slashes.

Duplicate paths are also rejected.

Validation

The manifest validates:

Manifest version
File paths
File sizes
Chunk object IDs
Duplicate paths
Repository-relative paths

Chunk IDs must currently use Forge's V1 object ID format:

blake3:<64-character lowercase hexadecimal digest>

Serialization

V1 manifests use JSON for their canonical representation.

The manifest supports:

Marshal
Unmarshal


and round-trip serialization is tested.

Immutability

AddFile copies the supplied chunk ID slice so callers cannot mutate a manifest indirectly through the original slice.

Clone produces an independent manifest copy.

Tests

The manifest package tests:

Manifest creation
File insertion
Path normalization
Duplicate paths
Deterministic serialization
Marshal/unmarshal round trips
Invalid paths
Invalid chunk IDs
Invalid file sizes
Unsupported versions
Empty manifests
Chunk ordering
Slice isolation
Manifest equality

Both normal tests and the race detector pass:

go test ./...

ok  	forge/internal/chunk
ok  	forge/internal/hash
ok  	forge/internal/manifest
ok  	forge/internal/object

go test -race ./...

ok  	forge/internal/chunk
ok  	forge/internal/hash
ok  	forge/internal/manifest
ok  	forge/internal/object

Architecture After Manifest

Forge's current foundation is:

Hashing
   ↓
Object Storage
   ↓
Chunking
   ↓
Manifest


The manifest currently describes the filesystem state but is not yet persisted as an object.

Next Task

Connect manifests to the content-addressed object store.

The next layer will:

Serialize a manifest canonically
Store the serialized manifest as an immutable object
Calculate its Forge object ID
Retrieve manifests by ID
Verify the stored manifest's integrity
Deduplicate identical manifests

This establishes the first complete content-addressed filesystem snapshot.

The next expected commit will be:

feat: store manifests as objects

:::

Then:

```bash
gofmt -w internal/manifest/manifest.go internal/manifest/manifest_test.go
git add DEVELOPMENT.md internal/manifest
git commit -m "feat: implement manifests"


Verify:

git log --oneline -3

2. Now connect Manifest → Object Store

Before I give you the next code, there's one important thing: I need to match the API of your actual internal/object/store.go.

We know it has a Put method because the chunker is using:

c.store.Put(...)


but I don't want to guess the rest of your object-store API and have you go through another correction cycle.

Run:

cat internal/object/store.go


and paste the output here.

Then I'll give you the exact manifest-storage implementation that matches your existing object store, rather than inventing an incompatible API.


---

## 2026-08-31 — Commit Storage

### Implemented

Built the Forge commit storage layer:

```text
internal/commit/
├── commit.go
├── commit_test.go
├── store.go
└── store_test.go


The commit layer provides an immutable representation of repository history and persists commits through the existing content-addressed object store.

Commit Storage

The commit store provides:

Commit serialization
Commit deserialization
Content-derived commit IDs
Commit storage
Commit retrieval
Commit existence checks
Commit integrity verification
Corruption detection
Missing commit detection
Concurrent duplicate storage

Commits are stored as ordinary Forge content-addressed objects.

Conceptually:

Commit
   ↓
Marshal
   ↓
Content bytes
   ↓
BLAKE3
   ↓
Forge object ID
   ↓
Object Store


This keeps commits immutable and allows the existing object-store integrity guarantees to apply to repository history.

Validation

Commit storage tests cover:

Put and Get
Deduplication
Existence checks
Missing commits
Integrity verification
Corruption detection
Invalid commits
Concurrent duplicate writes
Parent-child commit relationships
Test Results

Normal tests:

go test ./...

ok    forge/internal/chunk
ok    forge/internal/commit
ok    forge/internal/hash
ok    forge/internal/manifest
ok    forge/internal/object


Race detector:

go test -race ./...

ok    forge/internal/chunk
ok    forge/internal/commit
ok    forge/internal/hash
ok    forge/internal/manifest
ok    forge/internal/object


All tests pass.

Architectural State

Forge now has the following completed local storage primitives:

Hashing
   ✓
   ↓
Object Store
   ✓
   ↓
Chunking
   ✓
   ↓
Manifest
   ✓
   ↓
Manifest Store
   ✓
   ↓
Commit Model
   ✓
   ↓
Commit Store
   ✓


The next layer is the repository integration layer.

Next Task
Repository Integration

Build:

internal/repository/


The repository layer will compose the existing storage primitives into a complete local Forge repository.

Initial responsibilities:

Repository initialization
Repository opening
.forge/ directory layout
Object store initialization
Manifest store initialization
Commit store initialization
Repository metadata
Repository validation

Target layout:

project/
├── user files
└── .forge/
    ├── objects/
    ├── refs/
    └── config


The repository layer should remain local and offline.

It should not introduce networking, remotes, authentication, or other distributed functionality.

The immediate goal is to prove that the existing primitives can be safely composed into a single Forge repository.

:::

Then save and run:

```bash
gofmt -w internal/commit/*.go


DEVELOPMENT.md is Markdown, so gofmt does not apply to it. Just save the file normally.

Then verify:

git diff -- DEVELOPMENT.md


And finally:

go test ./...
go test -race ./...


If both remain green, commit this development-log update before starting repository implementation:

git add DEVELOPMENT.md
git commit -m "docs: record commit storage milestone"


Then:

git status


We want the repository clean again before writing the next layer.
## 2026-09-02 — Repository Integration

### Implemented

Built the Forge repository integration layer:

```text
internal/repository/
├── repository.go
└── repository_test.go

## 2026-09-02 — CLI Foundation and `init` Command

### Implemented

Created the Forge command-line interface entry point:

```text
cmd/forge/
└── main.go