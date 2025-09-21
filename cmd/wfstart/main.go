package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"go.temporal.io/sdk/client"

	"github.com/onasunnymorning/shadow-domain-ledger/temporal"
)

var (
	temporalClient client.Client
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "wfstart",
	Short: "Shadow Domain Ledger Workflow Starter",
	Long: `A CLI tool to start various workflows in the Shadow Domain Ledger system.
	
This tool provides convenient commands to trigger different workflows:
- mintDomains: Start the domain ingestion and NFT minting workflow
- hcsDemo: Start the HCS (Hedera Consensus Service) demonstration workflow`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Load .env file
		err := godotenv.Load()
		if err != nil {
			log.Println("No .env file found, relying on environment variables")
		}

		// Create a new Temporal client
		temporalClient, err = client.Dial(client.Options{})
		if err != nil {
			log.Fatalf("Unable to create Temporal client: %v", err)
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if temporalClient != nil {
			temporalClient.Close()
		}
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// mintDomainsCmd represents the mintDomains command
var mintDomainsCmd = &cobra.Command{
	Use:   "mintDomains [file]",
	Short: "Start the domain ingestion and NFT minting workflow",
	Long: `Start the domain ingestion workflow that reads domain events from a file,
parses them, groups by zones, and mints NFTs for each domain.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			log.Fatalf("File does not exist: %s", filePath)
		}

		// Workflow options
		workflowOptions := client.StartWorkflowOptions{
			ID:        "domain-ingest-workflow_" + filePath,
			TaskQueue: temporal.IngestTaskQueue,
		}

		// Execute the workflow
		we, err := temporalClient.ExecuteWorkflow(context.Background(), workflowOptions, temporal.IngestFileWorkflow, filePath)
		if err != nil {
			log.Fatalf("Unable to execute workflow: %v", err)
		}

		fmt.Printf("Started workflow - WorkflowID: %s, RunID: %s\n", we.GetID(), we.GetRunID())

		// Wait for the workflow to complete
		var result string
		err = we.Get(context.Background(), &result)
		if err != nil {
			log.Fatalf("Unable to get workflow result: %v", err)
		}
		fmt.Printf("Workflow completed. Result: %s\n", result)
	},
}

// hcsDemoCmd represents the hcsDemo command
var hcsDemoCmd = &cobra.Command{
	Use:   "hcsDemo [topicName]",
	Short: "Start the HCS demonstration workflow",
	Long: `Start the HCS (Hedera Consensus Service) demonstration workflow that
creates a topic, sends messages, and demonstrates subscription functionality.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		topicName := args[0]

		// Workflow options
		workflowOptions := client.StartWorkflowOptions{
			ID:        "hcs-demo-workflow_" + topicName,
			TaskQueue: temporal.IngestTaskQueue,
		}

		// Execute the workflow
		we, err := temporalClient.ExecuteWorkflow(context.Background(), workflowOptions, temporal.HCSDemoWorkflow, topicName)
		if err != nil {
			log.Fatalf("Unable to execute workflow: %v", err)
		}

		fmt.Printf("Started workflow - WorkflowID: %s, RunID: %s\n", we.GetID(), we.GetRunID())

		// Wait for the workflow to complete
		var result string
		err = we.Get(context.Background(), &result)
		if err != nil {
			log.Fatalf("Unable to get workflow result: %v", err)
		}
		fmt.Printf("Workflow completed. Result: %s\n", result)
	},
}

func init() {
	// Add subcommands
	rootCmd.AddCommand(mintDomainsCmd)
	rootCmd.AddCommand(hcsDemoCmd)
}
