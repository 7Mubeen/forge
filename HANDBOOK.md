# Forge Handbook

> **Forge is an open-source system for versioning, storing, and distributing massive datasets and binary artifacts.**

---

# 1. Project Vision

Forge exists to make version control practical for data that is too large or inefficient for traditional Git workflows.

Traditional Git is exceptionally good at versioning source code and relatively small files.

Forge is designed for data such as:

* Machine-learning datasets
* Scientific datasets
* Large binary assets
* Video and media
* Game assets
* Satellite/geospatial data
* Large model weights
* Simulation outputs
* Other multi-gigabyte or terabyte-scale data

Forge should make large data feel as natural to version as source code.

The long-term vision is:

```text
Git → version control for source code

Forge → version control + efficient distribution for massive data
```

---

# 2. The Fundamental Idea

Forge separates **history** from **data storage**.

Conceptually:

```text
                    Repository
                        │
                     History
                        │
                      Commit
                        │
                    Manifest
                        │
                  File structure
                        │
                      Chunks
                        │
                Content-addressed
                     objects
```

Git-like history describes *which version exists*.

Forge's object system describes *what data that version contains*.

---

# 3. The Core Problem

Large datasets create several problems for traditional version-control workflows.

A dataset might look like:

```text
dataset/
├── images/        3 TB
├── labels/        50 GB
├── metadata/      10 GB
└── models/        800 GB
```

A user may need to:

* Create multiple versions
* Avoid storing identical data repeatedly
* Download only part of a dataset
* Resume interrupted downloads
* Verify that data has not been corrupted
* Distribute data efficiently
* Reproduce an exact historical version

Forge is designed around these requirements.

---

# 4. Core Principles

## 4.1 Content Addressing

Objects are identified by the cryptographic hash of their contents.

```text
content
   ↓
hash
   ↓
object ID
```

The object's identity comes from its contents rather than its location.

The same content should produce the same object ID.

This provides the foundation for:

* Deduplication
* Integrity verification
* Immutable objects
* Distributed storage
* Peer-to-peer distribution

---

## 4.2 Immutability

Stored objects are immutable.

Once an object exists:

```text
object ID → content
```

that mapping must never change.

If the content changes, a new object is created.

---

## 4.3 Deduplication

Identical data should only need to be stored once.

Example:

```text
Version 1:
A B C D

Version 2:
A B C E

Version 3:
A B F E
```

Forge stores:

```text
A B C D E F
```

rather than storing complete copies of every version.

---

## 4.4 Streaming

Forge must be designed for large data.

A 10 TB file must never need to be loaded entirely into memory.

Operations should use streaming I/O:

```text
Disk
 ↓
Stream
 ↓
Chunk
 ↓
Hash
 ↓
Store
```

Memory usage should remain bounded regardless of file size.

---

## 4.5 Integrity

Forge must be able to detect corrupted data.

Objects are verified using their content hashes.

Conceptually:

```text
received object
       ↓
calculate hash
       ↓
compare with object ID
       ↓
   ┌───┴───┐
 valid    invalid
   ↓         ↓
accept     reject
```

---

## 4.6 Open Infrastructure

Forge must not fundamentally depend on Forge-operated servers.

The architecture should eventually support:

```text
Local storage
      │
      ├── Object storage
      ├── Self-hosted servers
      └── Peer-to-peer network
```

Users and organizations should be able to operate their own infrastructure.

---

# 5. Forge and Git

Forge does not attempt to replace Git's strengths.

Git provides excellent:

* Commits
* Branches
* Tags
* History
* Merging
* Source-code workflows

Forge focuses on the large-data problem.

The intended relationship is:

```text
              Git
               │
        source/history
               │
               ↓
             Forge
               │
        large objects
               │
       ┌───────┼────────┐
       ↓       ↓        ↓
     Local   Remote     P2P
```

Git interoperability is a long-term goal.

---

# 6. Forge V1

V1 is deliberately local and offline.

V1 must prove that the core data model works before networking is introduced.

### V1 includes

* Content-addressed object storage
* Cryptographic hashing
* File chunking
* Deduplication
* Manifests
* Commits
* Version history
* Checkout
* Status
* Integrity verification
* Garbage collection
* Streaming I/O

### V1 does NOT include

* P2P networking
* Remote repositories
* Cloud hosting
* Forge servers
* Authentication
* User accounts
* Web UI
* Encryption
* GitHub integration
* Distributed consensus

These belong to later stages.

---

# 7. V1 Architecture

The initial architecture should remain simple:

```text
                    Forge CLI
                       │
                    Core API
                       │
       ┌───────────────┼───────────────┐
       ↓               ↓               ↓
     Hash           Chunking       Repository
       │               │               │
       └───────────────┼───────────────┘
                       ↓
                  Object Store
                       │
                       ↓
                  Local Disk
```

The architecture must keep storage abstractions separate from the version-control model.

This allows future storage backends without redesigning the core.

---

# 8. V1 Repository Model

A Forge repository contains internal metadata and objects.

Conceptually:

```text
project/
├── user files
└── .forge/
    ├── objects/
    ├── refs/
    ├── commits/
    └── config
```

The exact on-disk format is an implementation decision, but it must be documented and stable.

---

# 9. Object Model

Forge will eventually have several types of objects.

Conceptually:

```text
Object
├── Chunk
├── File/Blob metadata
├── Manifest/Tree
└── Commit
```

Every immutable object should have a content-derived identity.

The object model should be designed so new object types can be introduced without breaking existing repositories.

---

# 10. Chunking

Large files are divided into chunks.

V1 may begin with fixed-size chunks:

```text
File
├── Chunk 1
├── Chunk 2
├── Chunk 3
└── ...
```

The chunking interface should remain replaceable so that future versions can introduce content-defined chunking.

Content-defined chunking may provide better deduplication when data is inserted or removed from the middle of large files.

---

# 11. Manifest

A manifest describes a filesystem state.

Conceptually:

```text
Manifest
│
├── file A
│   └── chunks
│
├── file B
│   └── chunks
│
└── directory C
    └── files
```

A manifest must allow Forge to reconstruct the exact filesystem state represented by a commit.

---

# 12. Commit

A commit represents an immutable version.

Conceptually:

```text
Commit
├── ID
├── Parent
├── Timestamp
├── Author
├── Message
└── Root Manifest
```

Commits form a history:

```text
Commit 1
   ↓
Commit 2
   ↓
Commit 3
```

Future versions may support branching and merging.

---

# 13. CLI Philosophy

Forge should feel familiar to Git users.

Initial commands:

```text
forge init
forge add
forge commit
forge status
forge log
forge checkout
forge fsck
forge gc
```

The CLI should remain small and composable.

---

# 14. Long-Term Roadmap

Forge development should progress in layers.

```text
V1
Local content-addressed versioning
        ↓
V2
Remote storage + push/pull
        ↓
V3
Partial and lazy fetching
        ↓
V4
Peer-to-peer distribution
        ↓
V5
Security + encryption
        ↓
V6
Git interoperability
        ↓
V7
Production distributed infrastructure
```

Each stage must build upon the previous stage.

---

# 15. P2P Vision

Peer-to-peer distribution is a long-term goal, not a V1 requirement.

The intended model is:

```text
                 Dataset
                    │
           ┌────────┼────────┐
           ↓        ↓        ↓
         Server   Peer A   Peer B
                    │
                ┌───┴───┐
                ↓       ↓
              Peer C  Peer D
```

Objects are verified by content hashes.

A peer does not need to be trusted to provide correct data.

If a peer provides an object whose hash does not match the requested object ID, the object is rejected.

---

# 16. What Forge Should NOT Become

Forge should not become:

* A social network
* A general-purpose cloud provider
* A proprietary hosted storage service
* A replacement for every existing version-control system
* A system that requires Forge's servers
* A complicated framework with unnecessary abstractions

The project should remain focused on its fundamental problem:

> **Efficiently versioning and distributing enormous amounts of data.**

---

# 17. Engineering Philosophy

Forge should prioritize:

1. Correctness
2. Simplicity
3. Data integrity
4. Reproducibility
5. Performance
6. Extensibility
7. Security

Performance must not come at the cost of correctness.

Complexity should only be introduced when a real requirement demands it.

---

# 18. V1 Definition of Done

V1 is complete when Forge can:

```text
Initialize a repository
        ↓
Add large files
        ↓
Chunk them
        ↓
Hash them
        ↓
Deduplicate them
        ↓
Create a manifest
        ↓
Create a commit
        ↓
Create another version
        ↓
View history
        ↓
Checkout either version
        ↓
Verify repository integrity
        ↓
Garbage-collect unreachable objects
```

All of this must work without a network connection.

---

# 19. The First Rule

Do not build the future before proving the present.

Before building P2P:

> Prove local content addressing.

Before building distributed storage:

> Prove the object model.

Before building partial downloads:

> Prove chunking and manifests.

Before building Git interoperability:

> Prove Forge's own version model.

The foundation must be correct before the system becomes distributed.

---

# 20. The Question Forge Is Trying to Answer

The project ultimately asks:

> **What would version control look like if massive data were treated as a first-class citizen?**

That question—not a specific implementation—is the heart of Forge.
