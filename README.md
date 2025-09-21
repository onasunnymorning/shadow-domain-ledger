# Shadow Domain Ledger

A comprehensive system for managing domain registration events on the Hedera blockchain, featuring NFT minting for domains organized by zones and Hedera Consensus Service (HCS) integration.

## Overview

The Shadow Domain Ledger processes domain registration events, validates domain names, and mints NFTs representing domains on the Hedera blockchain. The system uses zone-based NFT collections where each zone (like `.build`, `.app`, etc.) has its own NFT collection, and individual domains are minted as NFTs within their respective zone collections.

## Features

### 🏗️ **Domain Processing**
- **Event Ingestion**: Reads domain registration events from log files
- **Domain Validation**: Local domain name validation with comprehensive rules
- **Zone-based Organization**: Groups domains by their zones (.build, .app, etc.)
- **Duplicate Prevention**: Uses Hedera mirror node API to prevent duplicate minting

### 🎨 **NFT Management** 
- **Zone Collections**: Each zone gets its own NFT collection on Hedera
- **Domain NFTs**: Individual domains are minted as NFTs within zone collections
- **Metadata**: Rich metadata including domain name, zone, and registration details
- **Registry System**: Persistent tracking of collections and domains

### 📡 **Hedera Consensus Service (HCS)**
- **Topic Management**: Create and manage HCS topics
- **Message Publishing**: Send structured messages to topics
- **Subscription Handling**: Subscribe to and read from topics
- **Demo Workflows**: Complete HCS integration examples

### ⚡ **Temporal Workflows**
- **Orchestration**: Uses Temporal for reliable workflow execution
- **Activity-based**: Modular activities for different operations
- **Error Handling**: Robust retry policies and error management
- **Scalability**: Designed for high-volume domain processing

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Log Files     │───▶│  Temporal        │───▶│   Hedera        │
│  (Domain Events)│    │  Workflows       │    │  Blockchain     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │                          │
                              ▼                          ▼
                       ┌──────────────┐           ┌─────────────┐
                       │   CLI Tool   │           │    HCS      │
                       │  (wfstart)   │           │   Topics    │
                       └──────────────┘           └─────────────┘
```

## Quick Start

### Prerequisites

1. **Go 1.25+** installed
2. **Temporal server** running (local or remote)
3. **Hedera testnet account** with credentials
4. **Environment variables** configured

### Environment Setup

Create a `.env` file in the project root:

```bash
HEDERA_OPERATOR_ID=0.0.YOUR_ACCOUNT_ID
HEDERA_OPERATOR_KEY=your_private_key_here
HEDERA_NETWORK=testnet
```

### Installation

1. Clone the repository:
```bash
git clone https://github.com/onasunnymorning/shadow-domain-ledger.git
cd shadow-domain-ledger
```

2. Install dependencies:
```bash
go mod download
```

3. Build the CLI:
```bash
go build -o wfstart ./cmd/wfstart
```

4. Build the worker:
```bash
go build -o worker ./cmd/worker
```

### Running the System

1. **Start the Temporal worker**:
```bash
./worker
```

2. **Process domain events**:
```bash
./wfstart mintDomains testdata/dotBuild-events-2025-08.head20.log
```

3. **Run HCS demo**:
```bash
./wfstart hcsDemo my-test-topic
```

## Commands

### CLI Tool (`wfstart`)

#### `mintDomains`
Processes domain registration events and mints NFTs:
```bash
./wfstart mintDomains [file_path]
```

**What it does:**
- Reads domain events from the specified file
- Parses and validates domain names
- Groups domains by zones
- Creates NFT collections for each zone (if needed)
- Mints NFTs for each domain
- Prevents duplicates using mirror node verification

#### `hcsDemo`
Demonstrates HCS functionality:
```bash
./wfstart hcsDemo [topic_name]
```

**What it does:**
- Creates or looks up an HCS topic
- Sends demonstration messages
- Shows subscription capabilities
- Demonstrates complete HCS integration

## Project Structure

```
├── cmd/
│   ├── api/           # REST API server (placeholder)
│   ├── starter/       # Legacy workflow starter
│   ├── wfstart/       # New CLI tool
│   └── worker/        # Temporal worker
├── temporal/
│   ├── activities.go  # Business logic activities
│   ├── shared.go      # Data structures
│   └── workflow.go    # Workflow definitions
├── pkg/
│   └── domain/        # Domain validation logic
├── testdata/          # Sample domain event files
└── helmcharts/        # Kubernetes deployment configs
```

## Key Components

### Activities (`temporal/activities.go`)

**Domain Processing:**
- `ReadFileActivity` - Read domain event files
- `ParseAndFilterEventsActivity` - Parse domain events
- `ValidateDomainActivity` - Validate domain names
- `CheckDuplicateActivity` - Prevent duplicate minting
- `MintNFTActivity` - Mint domain NFTs

**Zone Management:**
- `CheckZoneRegistryActivity` - Check existing zones
- `CreateNFTCollectionActivity` - Create zone collections
- `UpdateZoneRegistryActivity` - Update zone tracking

**HCS Operations:**
- `CreateTopicActivity` - Create HCS topics
- `SendMessageToTopicActivity` - Send messages
- `SubscribeToTopicActivity` - Subscribe to topics
- `LookupOrCreateTopicActivity` - Topic management

### Workflows (`temporal/workflow.go`)

- **`IngestFileWorkflow`** - Complete domain processing pipeline
- **`HCSDemoWorkflow`** - HCS functionality demonstration

### Domain Validation (`pkg/domain/`)

Comprehensive domain name validation including:
- Label validation
- String normalization
- ASCII validation
- Length checks
- Character restrictions

## Data Persistence

The system uses JSON files for persistent state:

- **`zone_collections.json`** - Tracks NFT collections by zone
- **`hcs_topics.json`** - Tracks HCS topics by name

## Development

### Running Tests

```bash
go test ./...
```

### Building All Components

```bash
go build ./...
```

### Adding New Activities

1. Add activity function to `temporal/activities.go`
2. Register activity in `cmd/worker/main.go`
3. Use activity in workflows in `temporal/workflow.go`

## Deployment

The project includes Helm charts for Kubernetes deployment in the `helmcharts/` directory.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For questions or support, please open an issue on GitHub.
