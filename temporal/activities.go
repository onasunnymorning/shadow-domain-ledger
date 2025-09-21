package temporal

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	hedera "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"
	"github.com/onasunnymorning/shadow-domain-ledger/pkg/domain"
)

const (
	RegistryIDPrefix = "APEX" // Prefix for our Registry e.g. "APEX" would result in zones named "APEX-<ZonePrefix>"
	ZonePrefix       = "ZONE" // Suffix for zone collections e.g. "ZONE" would result in "<RegistryIDPrefix>-<ZonePrefix>.<zone>"

	// Hedera Mirror Node API endpoints (testnet)
	MirrorNodeBaseURL = "https://testnet.mirrornode.hedera.com/api/v1"
)

// Mirror Node API response structures
type MirrorNodeNFT struct {
	TokenID      string `json:"token_id"`
	SerialNumber int64  `json:"serial_number"`
	Metadata     string `json:"metadata"`
	CreatedAt    string `json:"created_timestamp"`
}

type MirrorNodeNFTsResponse struct {
	NFTs  []MirrorNodeNFT `json:"nfts"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

// Activities struct holds our activity implementations.
type Activities struct{}

// tokenIDFromString parses "shard.realm.num" (optionally with checksum suffix) into a hedera.TokenID.
func tokenIDFromString(s string) (hedera.TokenID, error) {
	base := strings.TrimSpace(s)
	if base == "" {
		return hedera.TokenID{}, fmt.Errorf("empty token id")
	}
	base = strings.SplitN(base, "-", 2)[0]
	parts := strings.Split(base, ".")
	if len(parts) != 3 {
		return hedera.TokenID{}, fmt.Errorf("invalid token id format: %s", s)
	}
	shard, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return hedera.TokenID{}, fmt.Errorf("invalid shard: %w", err)
	}
	realm, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return hedera.TokenID{}, fmt.Errorf("invalid realm: %w", err)
	}
	num, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return hedera.TokenID{}, fmt.Errorf("invalid token number: %w", err)
	}
	return hedera.TokenID{
		Shard: shard,
		Realm: realm,
		Token: num,
	}, nil
}

// ReadFileActivity reads a file from disk and returns its lines.
func (a *Activities) ReadFileActivity(ctx context.Context, filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// ParseAndFilterEventsActivity filters for domain "create" events.
func (a *Activities) ParseAndFilterEventsActivity(ctx context.Context, lines []string) ([]MintingInfo, error) {
	var mintingInfos []MintingInfo

	for _, line := range lines {
		if !strings.HasPrefix(line, `"registry-event"`) {
			continue // Skip malformed lines
		}

		// The log lines are not perfectly formatted JSON, so we fix them
		jsonString := "{" + line + "}"

		var event RegistryEvent
		if err := json.Unmarshal([]byte(jsonString), &event); err != nil {
			// Log error but continue processing other lines
			fmt.Printf("could not unmarshal line: %s, error: %v\n", jsonString, err)
			continue
		}

		// We only care about 'create' events for minting
		// TODO: add explicit filtering when event schema provides an action/type field.
		info := MintingInfo{
			DomainName:       event.Event.DomainName,
			RegistrationTime: time.Now(),
			RegistrarID:      event.Event.RegistrarID,
			Zone:             event.Event.Zone,
			FullEventJSON:    jsonString,
		}
		mintingInfos = append(mintingInfos, info)
	}
	return mintingInfos, nil
}

// MintNFTActivity connects to Hedera and mints the NFT in the specified zone collection.
func (a *Activities) MintNFTActivity(ctx context.Context, info MintingInfo, zoneCollection ZoneCollectionInfo) error {
	fmt.Printf("Minting NFT for domain: %s in .%s zone collection\n", info.DomainName, info.Zone)

	// --- Check if domain is already minted ---
	fmt.Printf("Checking if domain %s is already minted in collection %s...\n", info.DomainName, zoneCollection.TokenID)
	alreadyMinted, existingNFT, err := a.isDomainAlreadyMinted(info.DomainName, zoneCollection)
	if err != nil {
		fmt.Printf("Warning: Could not check mirror node for existing domain: %v. Proceeding with minting.\n", err)
	} else if alreadyMinted {
		fmt.Printf("Domain %s already minted as serial %d in collection %s (created %s). Skipping duplicate mint.\n",
			info.DomainName, existingNFT.SerialNumber, existingNFT.TokenID, existingNFT.CreatedAt)
		return nil // Return success since the domain is already minted
	}
	fmt.Printf("No existing NFT found for domain %s, proceeding with mint.\n", info.DomainName)

	// --- Load Hedera Credentials ---
	accountID, err := hedera.AccountIDFromString(os.Getenv("HEDERA_ACCOUNT_ID"))
	if err != nil {
		return fmt.Errorf("invalid HEDERA_ACCOUNT_ID: %w", err)
	}
	privateKey, err := hedera.PrivateKeyFromString(os.Getenv("HEDERA_PRIVATE_KEY"))
	if err != nil {
		return fmt.Errorf("invalid HEDERA_PRIVATE_KEY: %w", err)
	}

	// --- Parse the zone collection token ID ---
	tokenID, err := tokenIDFromString(zoneCollection.TokenID)
	if err != nil {
		return fmt.Errorf("invalid zone collection token ID: %w", err)
	}

	// --- Create Hedera Client ---
	client := hedera.ClientForTestnet()
	client.SetOperator(accountID, privateKey)

	// --- Prepare Metadata ---
	// For production, upload this to IPFS/Arweave and use the CID here.
	// For now, we'll use just the domain label since the zone is provided by the collection context
	dn, err := domain.NewDomainName(info.DomainName)
	if err != nil {
		return fmt.Errorf("failed to create domain name: %w", err)
	}
	metadata := []byte(dn.Label())
	fmt.Printf("Using metadata: '%s' (label only) for domain %s in .%s collection\n", dn.Label(), info.DomainName, info.Zone)

	// --- Mint Transaction ---
	mintTx := hedera.NewTokenMintTransaction().
		SetTokenID(tokenID).
		SetMetadata(metadata).
		SetMaxTransactionFee(hedera.NewHbar(20)) // Set a high max fee for assurance

	// Sign and execute
	txResponse, err := mintTx.Execute(client)
	if err != nil {
		return fmt.Errorf("transaction execution failed: %w", err)
	}

	// Get the receipt to confirm success
	receipt, err := txResponse.GetReceipt(client)
	if err != nil {
		return fmt.Errorf("failed to get transaction receipt: %w", err)
	}

	fmt.Printf("Successfully minted NFT for %s in .%s collection (token ID: %s). New serial: %d\n",
		info.DomainName, info.Zone, zoneCollection.TokenID, receipt.SerialNumbers[0])

	fmt.Printf("Domain %s is now recorded on Hedera blockchain and will be detected by mirror node queries\n", info.DomainName)

	return nil
}

// LookupOrCreateZoneCollectionActivity looks up an existing NFT collection for a zone,
// or creates a new one if it doesn't exist. Uses a registry file to track collections.
func (a *Activities) LookupOrCreateZoneCollectionActivity(ctx context.Context, zone string) (ZoneCollectionInfo, error) {
	fmt.Printf("Looking up or creating NFT collection for zone: .%s\n", zone)

	// Load the zone registry
	registry, err := a.loadZoneRegistry()
	if err != nil {
		fmt.Printf("Warning: Could not load zone registry: %v. Will check for existing collections anyway.\n", err)
		registry = &ZoneRegistry{
			Collections: make(map[string]ZoneCollectionInfo),
			LastUpdated: time.Now(),
		}
	}

	// Check if we already have this zone in our registry
	if collection, exists := registry.Collections[zone]; exists {
		fmt.Printf("Found existing NFT collection for .%s zone in registry: %s\n", zone, collection.TokenID)
		// Validate that the token still exists on Hedera
		if a.validateTokenExists(collection.TokenID) {
			return collection, nil
		} else {
			fmt.Printf("Warning: Token %s for zone .%s no longer exists on Hedera. Removing from registry.\n", collection.TokenID, zone)
			delete(registry.Collections, zone)
		}
	}

	// Search for existing collections by token name pattern
	fmt.Printf("Searching Hedera for existing .%s zone collections...\n", zone)
	existingCollection, found := a.searchForZoneCollection(zone)
	if found {
		fmt.Printf("Found existing .%s collection on Hedera: %s\n", zone, existingCollection.TokenID)
		// Add to registry for future lookups
		registry.Collections[zone] = existingCollection
		a.saveZoneRegistry(registry)
		return existingCollection, nil
	}

	// No existing collection found, create a new one
	fmt.Printf("No existing collection found for .%s zone, creating new collection...\n", zone)
	newCollection, err := a.CreateNFTCollectionActivity(ctx, zone)
	if err != nil {
		return ZoneCollectionInfo{}, err
	}

	// Add the new collection to the registry
	registry.Collections[zone] = newCollection
	registry.LastUpdated = time.Now()
	a.saveZoneRegistry(registry)

	return newCollection, nil
}

// loadZoneRegistry loads the zone registry from a JSON file
func (a *Activities) loadZoneRegistry() (*ZoneRegistry, error) {
	data, err := os.ReadFile(ZoneRegistryFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &ZoneRegistry{
				Collections: make(map[string]ZoneCollectionInfo),
				LastUpdated: time.Now(),
			}, nil
		}
		return nil, err
	}

	var registry ZoneRegistry
	err = json.Unmarshal(data, &registry)
	if err != nil {
		return nil, err
	}

	return &registry, nil
}

// saveZoneRegistry saves the zone registry to a JSON file
func (a *Activities) saveZoneRegistry(registry *ZoneRegistry) error {
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ZoneRegistryFile, data, 0644)
}

// validateTokenExists checks if a token ID still exists on Hedera
func (a *Activities) validateTokenExists(tokenID string) bool {
	// For now, just validate the format. In production, you could query Hedera mirror node
	_, err := tokenIDFromString(tokenID)
	return err == nil
}

// searchForZoneCollection searches for existing collections with our naming pattern
func (a *Activities) searchForZoneCollection(zone string) (ZoneCollectionInfo, bool) {
	// This is a simplified version. In production, you would:
	// 1. Query Hedera mirror node for tokens created by your account
	// 2. Filter by token name pattern: "Shadow Domain Ledger - .{zone}"
	// 3. Return the matching collection

	// For now, we'll return false to indicate no existing collection found
	// You can implement mirror node querying here if needed
	return ZoneCollectionInfo{}, false
}

// isDomainAlreadyMinted checks if a domain has already been minted by querying Hedera mirror nodes
// Uses smart pagination with early termination to avoid loading all NFTs
func (a *Activities) isDomainAlreadyMinted(domainName string, zoneCollection ZoneCollectionInfo) (bool, MirrorNodeNFT, error) {
	// Parse the domain name for comparison
	dn, err := domain.NewDomainName(domainName)
	if err != nil {
		return false, MirrorNodeNFT{}, fmt.Errorf("invalid domain name: %w", err)
	}
	expectedLabel := dn.Label()
	fmt.Printf("Checking for existing domain label: '%s' in collection %s\n", expectedLabel, zoneCollection.TokenID)

	// Use smart search with early termination
	foundNFT, found, err := a.searchForDomainInCollection(zoneCollection.TokenID, expectedLabel)
	if err != nil {
		return false, MirrorNodeNFT{}, fmt.Errorf("failed to search collection: %w", err)
	}

	if found {
		fmt.Printf("Found existing NFT for domain %s: Serial %d in collection %s\n",
			domainName, foundNFT.SerialNumber, foundNFT.TokenID)
		return true, foundNFT, nil
	}

	fmt.Printf("No existing NFT found for domain %s\n", domainName)
	return false, MirrorNodeNFT{}, nil
}

// searchForDomainInCollection performs an efficient search with early termination
func (a *Activities) searchForDomainInCollection(tokenID, expectedLabel string) (MirrorNodeNFT, bool, error) {
	const maxPagesToCheck = 50 // Limit search scope to prevent excessive API calls
	const pageSize = 100       // Reasonable page size

	client := &http.Client{Timeout: 30 * time.Second}

	// Start with newest NFTs first (more likely to find recent duplicates)
	nextURL := fmt.Sprintf("%s/tokens/%s/nfts?limit=%d&order=desc", MirrorNodeBaseURL, tokenID, pageSize)
	pagesChecked := 0

	for nextURL != "" && pagesChecked < maxPagesToCheck {
		fmt.Printf("Searching page %d of collection %s...\n", pagesChecked+1, tokenID)

		resp, err := client.Get(nextURL)
		if err != nil {
			return MirrorNodeNFT{}, false, fmt.Errorf("failed to query mirror node: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			// Collection doesn't exist yet or has no NFTs
			fmt.Printf("Collection %s not found or has no NFTs\n", tokenID)
			return MirrorNodeNFT{}, false, nil
		}

		if resp.StatusCode != http.StatusOK {
			return MirrorNodeNFT{}, false, fmt.Errorf("mirror node returned status %d", resp.StatusCode)
		}

		var response MirrorNodeNFTsResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return MirrorNodeNFT{}, false, fmt.Errorf("failed to decode mirror node response: %w", err)
		}

		fmt.Printf("Checking %d NFTs in page %d...\n", len(response.NFTs), pagesChecked+1)

		// Check each NFT in this page
		for i, nft := range response.NFTs {
			actualMetadata := strings.TrimSpace(nft.Metadata)

			// Try to decode base64 metadata
			var decodedMetadata string
			if decoded, err := base64.StdEncoding.DecodeString(actualMetadata); err == nil {
				decodedMetadata = string(decoded)
			} else {
				decodedMetadata = actualMetadata
			}

			fmt.Printf("  NFT %d: Serial %d, Metadata: '%s'\n", i+1, nft.SerialNumber, decodedMetadata)

			// Early termination: found a match!
			if decodedMetadata == expectedLabel || actualMetadata == expectedLabel {
				fmt.Printf("✓ Found match! Label '%s' exists as serial %d\n", expectedLabel, nft.SerialNumber)
				return nft, true, nil
			}
		}

		// Prepare for next page
		pagesChecked++
		if response.Links.Next != "" && pagesChecked < maxPagesToCheck {
			parsedURL, err := url.Parse(response.Links.Next)
			if err != nil {
				fmt.Printf("Warning: Could not parse next URL, stopping pagination\n")
				break
			}
			nextURL = fmt.Sprintf("%s%s", MirrorNodeBaseURL, parsedURL.RequestURI())
		} else {
			nextURL = ""
		}
	}

	if pagesChecked >= maxPagesToCheck {
		fmt.Printf("⚠️  Reached page limit (%d pages), assuming domain is new (collection may be very large)\n", maxPagesToCheck)
	}

	return MirrorNodeNFT{}, false, nil
}

// queryCollectionNFTs queries the Hedera mirror node for all NFTs in a collection
func (a *Activities) queryCollectionNFTs(tokenID string) ([]MirrorNodeNFT, error) {
	var allNFTs []MirrorNodeNFT
	nextURL := fmt.Sprintf("%s/tokens/%s/nfts?limit=100", MirrorNodeBaseURL, tokenID)

	client := &http.Client{Timeout: 30 * time.Second}

	for nextURL != "" {
		resp, err := client.Get(nextURL)
		if err != nil {
			return nil, fmt.Errorf("failed to query mirror node: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			// Collection doesn't exist yet or has no NFTs
			return []MirrorNodeNFT{}, nil
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("mirror node returned status %d", resp.StatusCode)
		}

		var response MirrorNodeNFTsResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, fmt.Errorf("failed to decode mirror node response: %w", err)
		}

		allNFTs = append(allNFTs, response.NFTs...)

		// Check for pagination
		if response.Links.Next != "" {
			// Parse the next URL - it comes as a full URL from mirror node
			parsedURL, err := url.Parse(response.Links.Next)
			if err != nil {
				break // Stop pagination on URL parse error
			}
			nextURL = fmt.Sprintf("%s%s", MirrorNodeBaseURL, parsedURL.RequestURI())
		} else {
			nextURL = ""
		}
	}

	return allNFTs, nil
}

// CheckCollectionNFTsActivity provides information about minted domains by querying mirror nodes
func (a *Activities) CheckCollectionNFTsActivity(ctx context.Context, tokenID string) error {
	fmt.Printf("=== Checking NFTs in Collection %s ===\n", tokenID)

	nfts, err := a.queryCollectionNFTs(tokenID)
	if err != nil {
		fmt.Printf("Error querying collection NFTs: %v\n", err)
		return err
	}

	fmt.Printf("Total NFTs in collection: %d\n", len(nfts))

	if len(nfts) > 0 {
		fmt.Println("Minted NFTs:")
		for _, nft := range nfts {
			fmt.Printf("  - Serial %d: %s (created %s)\n",
				nft.SerialNumber, nft.Metadata, nft.CreatedAt)
		}
	}

	fmt.Println("=== End Collection Check ===")
	return nil
}

// DebugEnvironmentActivity prints all environment variables starting with HEDERA_NFT_ID
func (a *Activities) DebugEnvironmentActivity(ctx context.Context) error {
	fmt.Println("=== Debug: Environment Variables ===")
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "HEDERA_NFT_ID") {
			fmt.Printf("Found env var: %s\n", env)
		}
	}
	fmt.Println("=== End Debug ===")
	return nil
}

// CreateNFTCollectionActivity creates a new NFT collection for a specific zone on Hedera
func (a *Activities) CreateNFTCollectionActivity(ctx context.Context, zone string) (ZoneCollectionInfo, error) {
	fmt.Printf("Creating NFT collection for zone: .%s\n", zone)

	// --- Load Hedera Credentials ---
	accountID, err := hedera.AccountIDFromString(os.Getenv("HEDERA_ACCOUNT_ID"))
	if err != nil {
		return ZoneCollectionInfo{}, fmt.Errorf("invalid HEDERA_ACCOUNT_ID: %w", err)
	}
	privateKey, err := hedera.PrivateKeyFromString(os.Getenv("HEDERA_PRIVATE_KEY"))
	if err != nil {
		return ZoneCollectionInfo{}, fmt.Errorf("invalid HEDERA_PRIVATE_KEY: %w", err)
	}

	// --- Create Hedera Client ---
	client := hedera.ClientForTestnet()
	client.SetOperator(accountID, privateKey)

	// --- Create the NFT collection for this zone ---
	tokenName := fmt.Sprintf("%s Domain Ledger Zone - .%s", strings.ToUpper(RegistryIDPrefix), strings.ToUpper(zone))
	tokenSymbol := fmt.Sprintf("%s-%s.%s", strings.ToUpper(RegistryIDPrefix), strings.ToUpper(ZonePrefix), strings.ToUpper(zone))

	tokenCreateTx := hedera.NewTokenCreateTransaction().
		SetTokenName(tokenName).
		SetTokenSymbol(tokenSymbol).
		SetTokenType(hedera.TokenTypeNonFungibleUnique).
		SetDecimals(0).
		SetInitialSupply(0).
		SetTreasuryAccountID(accountID).
		SetSupplyType(hedera.TokenSupplyTypeInfinite).
		SetSupplyKey(privateKey).
		SetMaxTransactionFee(hedera.NewHbar(30))

	// Execute the transaction
	txResponse, err := tokenCreateTx.Execute(client)
	if err != nil {
		return ZoneCollectionInfo{}, fmt.Errorf("failed to execute token create transaction: %w", err)
	}

	// Get the receipt
	receipt, err := txResponse.GetReceipt(client)
	if err != nil {
		return ZoneCollectionInfo{}, fmt.Errorf("failed to get token create receipt: %w", err)
	}

	if receipt.TokenID == nil {
		return ZoneCollectionInfo{}, fmt.Errorf("token creation failed: no token ID in receipt")
	}

	tokenID := receipt.TokenID.String()
	fmt.Printf("Successfully created NFT collection for .%s zone with token ID: %s\n", zone, tokenID)
	fmt.Printf("Collection will be automatically tracked in registry for future reuse\n")

	return ZoneCollectionInfo{
		Zone:        zone,
		TokenID:     tokenID,
		TokenName:   tokenName,
		TokenSymbol: tokenSymbol,
		CreatedAt:   time.Now(),
		CreatedBy:   accountID.String(),
	}, nil
}

// ============================================================================
// HCS (Hedera Consensus Service) Activities
// ============================================================================

// CreateTopicActivity creates a new HCS topic on Hedera
func (a *Activities) CreateTopicActivity(ctx context.Context, topicName, description string, enableAdminKey, enableSubmitKey bool) (TopicInfo, error) {
	fmt.Printf("Creating HCS topic: %s\n", topicName)

	// --- Load Hedera Credentials ---
	accountID, err := hedera.AccountIDFromString(os.Getenv("HEDERA_ACCOUNT_ID"))
	if err != nil {
		return TopicInfo{}, fmt.Errorf("invalid HEDERA_ACCOUNT_ID: %w", err)
	}
	privateKey, err := hedera.PrivateKeyFromString(os.Getenv("HEDERA_PRIVATE_KEY"))
	if err != nil {
		return TopicInfo{}, fmt.Errorf("invalid HEDERA_PRIVATE_KEY: %w", err)
	}

	// --- Create Hedera Client ---
	client := hedera.ClientForTestnet()
	client.SetOperator(accountID, privateKey)

	// --- Create Topic Transaction ---
	topicCreateTx := hedera.NewTopicCreateTransaction().
		SetTopicMemo(description).
		SetMaxTransactionFee(hedera.NewHbar(5)) // Set reasonable fee

	// Optionally set admin key (allows topic updates/deletion)
	if enableAdminKey {
		topicCreateTx.SetAdminKey(privateKey.PublicKey())
	}

	// Optionally set submit key (restricts who can submit messages)
	if enableSubmitKey {
		topicCreateTx.SetSubmitKey(privateKey.PublicKey())
	}

	// Execute the transaction
	txResponse, err := topicCreateTx.Execute(client)
	if err != nil {
		return TopicInfo{}, fmt.Errorf("failed to execute topic create transaction: %w", err)
	}

	// Get the receipt
	receipt, err := txResponse.GetReceipt(client)
	if err != nil {
		return TopicInfo{}, fmt.Errorf("failed to get topic create receipt: %w", err)
	}

	if receipt.TopicID == nil {
		return TopicInfo{}, fmt.Errorf("topic creation failed: no topic ID in receipt")
	}

	topicID := receipt.TopicID.String()
	fmt.Printf("Successfully created HCS topic '%s' with ID: %s\n", topicName, topicID)

	topicInfo := TopicInfo{
		TopicID:     topicID,
		TopicName:   topicName,
		Description: description,
		CreatedAt:   time.Now(),
		CreatedBy:   accountID.String(),
	}

	if enableAdminKey {
		topicInfo.AdminKey = privateKey.PublicKey().String()
	}
	if enableSubmitKey {
		topicInfo.SubmitKey = privateKey.PublicKey().String()
	}

	// Store in topic registry for future use
	err = a.registerTopic(topicInfo)
	if err != nil {
		fmt.Printf("Warning: Could not register topic in registry: %v\n", err)
	}

	return topicInfo, nil
}

// SendMessageToTopicActivity sends a message to an HCS topic
func (a *Activities) SendMessageToTopicActivity(ctx context.Context, topicID, message string) (TopicMessage, error) {
	fmt.Printf("Sending message to topic %s: %s\n", topicID, message)

	// --- Load Hedera Credentials ---
	accountID, err := hedera.AccountIDFromString(os.Getenv("HEDERA_ACCOUNT_ID"))
	if err != nil {
		return TopicMessage{}, fmt.Errorf("invalid HEDERA_ACCOUNT_ID: %w", err)
	}
	privateKey, err := hedera.PrivateKeyFromString(os.Getenv("HEDERA_PRIVATE_KEY"))
	if err != nil {
		return TopicMessage{}, fmt.Errorf("invalid HEDERA_PRIVATE_KEY: %w", err)
	}

	// --- Parse Topic ID ---
	hederaTopicID, err := hedera.TopicIDFromString(topicID)
	if err != nil {
		return TopicMessage{}, fmt.Errorf("invalid topic ID: %w", err)
	}

	// --- Create Hedera Client ---
	client := hedera.ClientForTestnet()
	client.SetOperator(accountID, privateKey)

	// --- Send Message Transaction ---
	messageTx := hedera.NewTopicMessageSubmitTransaction().
		SetTopicID(hederaTopicID).
		SetMessage([]byte(message)).
		SetMaxTransactionFee(hedera.NewHbar(5))

	// Execute the transaction
	txResponse, err := messageTx.Execute(client)
	if err != nil {
		return TopicMessage{}, fmt.Errorf("failed to execute message submit transaction: %w", err)
	}

	// Get the receipt
	receipt, err := txResponse.GetReceipt(client)
	if err != nil {
		return TopicMessage{}, fmt.Errorf("failed to get message submit receipt: %w", err)
	}

	fmt.Printf("Successfully sent message to topic %s. Sequence number: %d\n", topicID, receipt.TopicSequenceNumber)

	return TopicMessage{
		TopicID:        topicID,
		SequenceNumber: receipt.TopicSequenceNumber,
		ConsensusTime:  time.Now(), // Approximate - real consensus time comes from mirror node
		Message:        message,
		RunningHash:    fmt.Sprintf("%x", receipt.TopicRunningHash), // Convert bytes to hex string
		PayerAccountID: accountID.String(),
	}, nil
}

// SubscribeToTopicActivity subscribes to an HCS topic and reads messages
func (a *Activities) SubscribeToTopicActivity(ctx context.Context, subscription TopicSubscriptionInfo) ([]TopicMessage, error) {
	fmt.Printf("Subscribing to topic %s\n", subscription.TopicID)

	// --- Parse Topic ID ---
	hederaTopicID, err := hedera.TopicIDFromString(subscription.TopicID)
	if err != nil {
		return nil, fmt.Errorf("invalid topic ID: %w", err)
	}

	// --- Create Hedera Client ---
	client := hedera.ClientForTestnet()

	var messages []TopicMessage
	messageCount := 0

	// Create subscription query
	query := hedera.NewTopicMessageQuery().
		SetTopicID(hederaTopicID).
		SetMaxAttempts(3)

	// Set start time if specified
	if !subscription.StartTime.IsZero() {
		query.SetStartTime(subscription.StartTime)
	}

	// Set end time if specified
	if !subscription.EndTime.IsZero() {
		query.SetEndTime(subscription.EndTime)
	}

	// Set limit if specified
	limit := subscription.Limit
	if limit == 0 {
		limit = 100 // Default limit to prevent runaway subscriptions
	}

	fmt.Printf("Starting subscription with limit: %d messages\n", limit)

	// Subscribe and handle messages
	_, err = query.Subscribe(client, func(message hedera.TopicMessage) {
		messageCount++
		fmt.Printf("Received message %d: Sequence %d at %s\n",
			messageCount, message.SequenceNumber, message.ConsensusTimestamp.Format(time.RFC3339))

		topicMsg := TopicMessage{
			TopicID:        subscription.TopicID,
			SequenceNumber: message.SequenceNumber,
			ConsensusTime:  message.ConsensusTimestamp,
			Message:        string(message.Contents),
			RunningHash:    fmt.Sprintf("%x", message.RunningHash), // Convert bytes to hex string
		}
		messages = append(messages, topicMsg)

		// Stop if we've reached the limit
		if messageCount >= limit {
			fmt.Printf("Reached message limit (%d), stopping subscription\n", limit)
			return
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	fmt.Printf("Subscription completed. Received %d messages\n", len(messages))
	return messages, nil
}

// LookupOrCreateTopicActivity looks up an existing topic or creates a new one
func (a *Activities) LookupOrCreateTopicActivity(ctx context.Context, topicName, description string, enableAdminKey, enableSubmitKey bool) (TopicInfo, error) {
	fmt.Printf("Looking up or creating HCS topic: %s\n", topicName)

	// Load the topic registry
	registry, err := a.loadTopicRegistry()
	if err != nil {
		fmt.Printf("Warning: Could not load topic registry: %v. Will create new topic.\n", err)
	} else {
		// Check if we already have this topic in our registry
		if topicInfo, exists := registry.Topics[topicName]; exists {
			fmt.Printf("Found existing topic '%s' in registry: %s\n", topicName, topicInfo.TopicID)
			return topicInfo, nil
		}
	}

	// No existing topic found, create a new one
	fmt.Printf("No existing topic found for '%s', creating new topic...\n", topicName)
	return a.CreateTopicActivity(ctx, topicName, description, enableAdminKey, enableSubmitKey)
}

// GetTopicInfoActivity retrieves information about a topic from the registry
func (a *Activities) GetTopicInfoActivity(ctx context.Context, topicName string) (TopicInfo, error) {
	registry, err := a.loadTopicRegistry()
	if err != nil {
		return TopicInfo{}, fmt.Errorf("failed to load topic registry: %w", err)
	}

	if topicInfo, exists := registry.Topics[topicName]; exists {
		return topicInfo, nil
	}

	return TopicInfo{}, fmt.Errorf("topic '%s' not found in registry", topicName)
}

// loadTopicRegistry loads the topic registry from a JSON file
func (a *Activities) loadTopicRegistry() (*TopicRegistry, error) {
	data, err := os.ReadFile(TopicRegistryFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &TopicRegistry{
				Topics:      make(map[string]TopicInfo),
				LastUpdated: time.Now(),
			}, nil
		}
		return nil, err
	}

	var registry TopicRegistry
	err = json.Unmarshal(data, &registry)
	if err != nil {
		return nil, err
	}

	return &registry, nil
}

// saveTopicRegistry saves the topic registry to a JSON file
func (a *Activities) saveTopicRegistry(registry *TopicRegistry) error {
	registry.LastUpdated = time.Now()
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(TopicRegistryFile, data, 0644)
}

// registerTopic adds a topic to the registry
func (a *Activities) registerTopic(topicInfo TopicInfo) error {
	registry, err := a.loadTopicRegistry()
	if err != nil {
		return err
	}

	registry.Topics[topicInfo.TopicName] = topicInfo
	return a.saveTopicRegistry(registry)
}

// CheckTopicRegistryActivity provides information about registered topics for debugging
func (a *Activities) CheckTopicRegistryActivity(ctx context.Context) error {
	fmt.Println("=== HCS Topic Registry Status ===")

	registry, err := a.loadTopicRegistry()
	if err != nil {
		fmt.Printf("Error loading topic registry: %v\n", err)
		return err
	}

	fmt.Printf("Total registered topics: %d\n", len(registry.Topics))
	fmt.Printf("Registry last updated: %s\n", registry.LastUpdated.Format(time.RFC3339))

	if len(registry.Topics) > 0 {
		fmt.Println("Registered topics:")
		for name, info := range registry.Topics {
			fmt.Printf("  - %s: %s (created %s)\n",
				name, info.TopicID, info.CreatedAt.Format(time.RFC3339))
		}
	}

	fmt.Println("=== End Topic Registry ===")
	return nil
}
