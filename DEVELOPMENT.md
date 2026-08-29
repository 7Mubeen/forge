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
