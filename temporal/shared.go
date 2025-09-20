package temporal

import "time"

const IngestTaskQueue = "DOMAIN_INGEST_TASK_QUEUE"

// EventData matches the structure of the JSON object inside the log file.
// We use json tags to map the JSON keys to our struct fields.
type EventData struct {
	Initiator   string `json:"i"`
	RegistrarID string `json:"r"`
	Type        string `json:"t"`
	DomainName  string `json:"o"`
	Event       string `json:"e"`
	Timestamp   string `json:"s"` // Keep as string for now, parse later
	Zone        string `json:"z"`
}

// RegistryEvent is the top-level object in each log line.
type RegistryEvent struct {
	Event EventData `json:"registry-event"`
}

// MintingInfo contains all the necessary data for the minting activity.
type MintingInfo struct {
	DomainName       string
	RegistrationTime time.Time
	RegistrarID      string
	Zone             string // The zone this domain belongs to (e.g., "build", "com", etc.)
	FullEventJSON    string // Store the original event for metadata
}

// ZoneCollectionInfo holds information about an NFT collection for a specific zone
type ZoneCollectionInfo struct {
	Zone        string    `json:"zone"`         // The zone name (e.g., "build", "com")
	TokenID     string    `json:"token_id"`     // Hedera token ID for this zone's collection
	TokenName   string    `json:"token_name"`   // Human readable token name
	TokenSymbol string    `json:"token_symbol"` // Token symbol
	CreatedAt   time.Time `json:"created_at"`   // When this collection was created
	CreatedBy   string    `json:"created_by"`   // Account ID that created this collection
}

// ZoneRegistry tracks all zone collections to avoid duplicates
type ZoneRegistry struct {
	Collections map[string]ZoneCollectionInfo `json:"collections"` // zone -> collection info
	LastUpdated time.Time                     `json:"last_updated"`
}

// ZoneRegistryFile is the file where we persist the zone registry
const ZoneRegistryFile = "zone_collections.json"
