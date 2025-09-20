package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/onasunnymorning/shadow-domain-ledger/temporal"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
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

	// Create a new worker
	w := worker.New(c, temporal.IngestTaskQueue, worker.Options{})

	// Register the Workflow and Activities
	w.RegisterWorkflow(temporal.IngestFileWorkflow)
	w.RegisterActivity(&temporal.Activities{})

	// Start listening to the Task Queue
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
