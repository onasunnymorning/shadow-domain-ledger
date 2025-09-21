package temporal

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// IngestFileWorkflow orchestrates the domain ingestion and minting process
func IngestFileWorkflow(ctx workflow.Context, filePath string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting domain ingestion workflow", "filePath", filePath)

	// Set up activity options
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// Step 1: Read the file
	var lines []string
	err := workflow.ExecuteActivity(ctx, "ReadFileActivity", filePath).Get(ctx, &lines)
	if err != nil {
		logger.Error("Failed to read file", "error", err)
		return err
	}
	logger.Info("Read file successfully", "lineCount", len(lines))

	// Step 2: Parse and filter events
	var mintingInfos []MintingInfo
	err = workflow.ExecuteActivity(ctx, "ParseAndFilterEventsActivity", lines).Get(ctx, &mintingInfos)
	if err != nil {
		logger.Error("Failed to parse events", "error", err)
		return err
	}
	logger.Info("Parsed events successfully", "eventCount", len(mintingInfos))

	// Step 3: Group domains by zone and process each zone
	zoneGroups := make(map[string][]MintingInfo)
	for _, info := range mintingInfos {
		zone := info.Zone
		zoneGroups[zone] = append(zoneGroups[zone], info)
	}

	logger.Info("Grouped domains by zone", "zoneCount", len(zoneGroups))

	// Step 4: Process each zone
	for zone, domainInfos := range zoneGroups {
		logger.Info("Processing zone", "zone", zone, "domainCount", len(domainInfos))

		// Look up or create the NFT collection for this zone
		var zoneCollection ZoneCollectionInfo
		err = workflow.ExecuteActivity(ctx, "LookupOrCreateZoneCollectionActivity", zone).Get(ctx, &zoneCollection)
		if err != nil {
			logger.Error("Failed to lookup/create zone collection", "zone", zone, "error", err)
			continue // Continue with other zones
		}

		// Mint NFTs for all domains in this zone
		for _, info := range domainInfos {
			err = workflow.ExecuteActivity(ctx, "MintNFTActivity", info, zoneCollection).Get(ctx, nil)
			if err != nil {
				logger.Error("Failed to mint NFT", "domain", info.DomainName, "zone", zone, "error", err)
				// Continue with other domains instead of failing the entire workflow
				continue
			}
			logger.Info("Successfully minted NFT", "domain", info.DomainName, "zone", zone)
		}
	}

	logger.Info("Completed domain ingestion workflow", "totalZones", len(zoneGroups))
	return nil
}

// HCSDemoWorkflow demonstrates HCS functionality with topic creation, messaging, and subscription
func HCSDemoWorkflow(ctx workflow.Context, topicName string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting HCS demo workflow", "topicName", topicName)

	// Set up activity options
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// Step 1: Create or lookup a topic
	var topicInfo TopicInfo
	err := workflow.ExecuteActivity(ctx, "LookupOrCreateTopicActivity",
		topicName,
		fmt.Sprintf("Demo topic for %s domain events", topicName),
		true,  // enableAdminKey
		false, // enableSubmitKey - allow anyone to send messages
	).Get(ctx, &topicInfo)
	if err != nil {
		logger.Error("Failed to create/lookup topic", "error", err)
		return err
	}
	logger.Info("Topic ready", "topicID", topicInfo.TopicID)

	// Step 2: Send some demo messages
	messages := []string{
		fmt.Sprintf("HCS Demo started for topic: %s", topicName),
		fmt.Sprintf("Topic ID: %s", topicInfo.TopicID),
		"This is a test message for domain event streaming",
		fmt.Sprintf("Demo completed at: %s", time.Now().Format(time.RFC3339)),
	}

	var sentMessages []TopicMessage
	for i, msg := range messages {
		var topicMsg TopicMessage
		err := workflow.ExecuteActivity(ctx, "SendMessageToTopicActivity", topicInfo.TopicID, msg).Get(ctx, &topicMsg)
		if err != nil {
			logger.Error("Failed to send message", "messageNum", i+1, "error", err)
			continue
		}
		sentMessages = append(sentMessages, topicMsg)
		logger.Info("Sent message", "messageNum", i+1, "sequenceNumber", topicMsg.SequenceNumber)

		// Wait a bit between messages
		workflow.Sleep(ctx, 2*time.Second)
	}

	// Step 3: Subscribe and read back the messages
	subscription := TopicSubscriptionInfo{
		TopicID:   topicInfo.TopicID,
		StartTime: time.Now().Add(-5 * time.Minute), // Start from 5 minutes ago
		Limit:     10,                               // Read up to 10 messages
	}

	var receivedMessages []TopicMessage
	err = workflow.ExecuteActivity(ctx, "SubscribeToTopicActivity", subscription).Get(ctx, &receivedMessages)
	if err != nil {
		logger.Error("Failed to subscribe to topic", "error", err)
		// Don't fail the workflow - subscription issues are not critical
	} else {
		logger.Info("Subscription completed", "messagesReceived", len(receivedMessages))
	}

	// Step 4: Show registry status
	err = workflow.ExecuteActivity(ctx, "CheckTopicRegistryActivity").Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to check topic registry", "error", err)
	}

	logger.Info("HCS demo workflow completed",
		"topicName", topicName,
		"topicID", topicInfo.TopicID,
		"messagesSent", len(sentMessages),
		"messagesReceived", len(receivedMessages))

	return nil
}
