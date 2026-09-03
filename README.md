# Forge

**Version control for massive datasets and binary artifacts.**

Forge is an open-source system designed to make version control practical for data that is too large or inefficient for traditional Git workflows.

## Why Forge?

Traditional Git excels at versioning source code and small files, but struggles with:
- Machine learning datasets (terabytes of training data)
- Scientific datasets (genomics, astronomy, climate simulations)
- Large binary assets (game assets, video, 3D models)
- Model weights and simulation outputs

Forge solves this by treating massive data as a **first-class citizen**, using content-addressed storage and intelligent chunking to efficiently version, deduplicate, and distribute enormous datasets.

## Key Features

### V1 (Complete)
- **Content-addressed storage** - Objects identified by cryptographic hash (BLAKE3)
- **Intelligent chunking** - Large files split into 4MB chunks for efficient deduplication
- **Immutable history** - Once committed, data cannot be silently altered
- **Integrity verification** - Detect corruption with `forge fsck`
- **Garbage collection** - Remove unreachable objects with `forge gc`
- **Time travel** - Checkout any historical version with `forge checkout`
- **Streaming architecture** - Handle 10TB+ files with bounded memory usage
- **Offline-first** - Works completely without network connectivity

### Roadmap
-  **V2**: Remote storage + push/pull
-  **V3**: Partial and lazy fetching
-  **V4**: Peer-to-peer distribution
-  **V5**: Security + encryption
-  **V6**: Git interoperability
-  **V7**: Production distributed infrastructure

## Installation

### From Source

Requires Go 1.27 or later:

```bash
git clone https://github.com/7Mubeen/forge.git
cd forge
go build -o forge ./cmd/forge