package temporal

import (
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
