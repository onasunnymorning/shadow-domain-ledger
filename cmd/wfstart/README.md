# wfstart CLI

A command-line interface for starting workflows in the Shadow Domain Ledger system.

## Installation

Build the CLI from the project root:

```bash
go build -o wfstart ./cmd/wfstart
```

## Usage

### Basic Help

```bash
./wfstart --help
```

### Commands

#### mintDomains

Start the domain ingestion and NFT minting workflow:

```bash
./wfstart mintDomains [file_path]
```

Example:
```bash
./wfstart mintDomains testdata/dotBuild-events-2025-08.head20.log
```

This command:
- Reads domain events from the specified file
- Parses and filters the events
- Groups domains by zones
- Creates NFT collections for each zone (if they don't exist)
- Mints NFTs for each domain

#### hcsDemo

Start the HCS (Hedera Consensus Service) demonstration workflow:

```bash
./wfstart hcsDemo [topic_name]
```

Example:
```bash
./wfstart hcsDemo my-test-topic
```

This command:
- Creates or looks up an HCS topic with the given name
- Sends demo messages to the topic
- Demonstrates subscription and message reading functionality
- Shows HCS integration capabilities

## Prerequisites

- Temporal server running (local or remote)
- Hedera testnet account configured (via environment variables)
- Valid `.env` file or environment variables set

## Environment Variables

Make sure you have the required Hedera environment variables set:

- `HEDERA_OPERATOR_ID`
- `HEDERA_OPERATOR_KEY`
- `HEDERA_NETWORK` (defaults to testnet)

## Notes

- The CLI will automatically load a `.env` file if present
- Both commands will wait for the workflow to complete and show the result
- Workflow IDs are generated based on the input parameters to avoid conflicts
