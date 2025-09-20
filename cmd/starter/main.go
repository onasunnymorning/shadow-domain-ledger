package main

import (
	"context"
	"log"

	"github.com/onasunnymorning/shadow-domain-ledger/temporal"

	"github.com/joho/godotenv"
	"go.temporal.io/sdk/client"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	// Create a new Temporal client
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	// The file to process
	filePath := "testdata/dotBuild-events-2025-08.head20.log"

	// Workflow options
	workflowOptions := client.StartWorkflowOptions{
		ID:        "domain-ingest-workflow_" + filePath,
		TaskQueue: temporal.IngestTaskQueue,
	}

	// Execute the workflow
	we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, temporal.IngestFileWorkflow, filePath)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	log.Println("Started workflow", "WorkflowID", we.GetID(), "RunID", we.GetRunID())

	// Wait for the workflow to complete
	var result string
	err = we.Get(context.Background(), &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}
	log.Printf("Workflow completed. Result: %s\n", result)
}
