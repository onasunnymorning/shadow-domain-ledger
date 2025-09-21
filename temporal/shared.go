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

// HCS-related structures

// TopicInfo holds information about an HCS topic
type TopicInfo struct {
	TopicID     string    `json:"topic_id"`    // Hedera topic ID (e.g., "0.0.123456")
	TopicName   string    `json:"topic_name"`  // Human readable topic name
	Description string    `json:"description"` // Topic description
	CreatedAt   time.Time `json:"created_at"`  // When this topic was created
	CreatedBy   string    `json:"created_by"`  // Account ID that created this topic
	AdminKey    string    `json:"admin_key"`   // Admin key for topic management (optional)
	SubmitKey   string    `json:"submit_key"`  // Submit key for message submission (optional)
}

// TopicMessage represents a message sent to an HCS topic
type TopicMessage struct {
	TopicID        string    `json:"topic_id"`         // Topic the message was sent to
	SequenceNumber uint64    `json:"sequence_number"`  // Message sequence number in topic
	ConsensusTime  time.Time `json:"consensus_time"`   // When consensus was reached
	Message        string    `json:"message"`          // The actual message content
	RunningHash    string    `json:"running_hash"`     // Topic running hash after this message
	PayerAccountID string    `json:"payer_account_id"` // Account that paid for the message
}

// TopicSubscriptionInfo holds subscription configuration
type TopicSubscriptionInfo struct {
	TopicID   string    `json:"topic_id"`   // Topic to subscribe to
	StartTime time.Time `json:"start_time"` // When to start reading from (optional)
	EndTime   time.Time `json:"end_time"`   // When to stop reading (optional)
	Limit     int       `json:"limit"`      // Max number of messages to read (optional)
}

// TopicRegistry tracks HCS topics to avoid duplicates and enable reuse
type TopicRegistry struct {
	Topics      map[string]TopicInfo `json:"topics"` // topic name -> topic info
	LastUpdated time.Time            `json:"last_updated"`
}

// TopicRegistryFile is the file where we persist the topic registry
const TopicRegistryFile = "hcs_topics.json"
