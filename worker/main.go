package main

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"workflow_engine/app"
)

func main() {
	// Create the client object just once per process
	c, err := client.NewClient(client.Options{})
	if err != nil {
		log.Fatalln("unable to create Temporal client", err)
	}
	defer c.Close()
	// This worker hosts both Worker and Activity functions
	w := worker.New(c, app.WorkflowEngineTaskQueue, worker.Options{})

	w.RegisterWorkflow(app.SimpleDSLWorkflow)
	w.RegisterActivity(&app.ActivityType{})

	err = w.Run(nil) // Don't stop on error
	if err != nil {
		log.Fatalln("unable to start Worker", err)
	}
}
