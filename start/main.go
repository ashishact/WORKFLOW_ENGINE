package main

import (
	"context"
	"fmt"
	"log"

	"go.temporal.io/sdk/client"

	"workflow_engine/app"
)

func main() {
	// Create the client object just once per process
	c, err := client.NewClient(client.Options{})
	if err != nil {
		log.Fatalln("unable to create Temporal client", err)
	}
	defer c.Close()
	options := client.StartWorkflowOptions{
		ID:        "mega-workflow",
		TaskQueue: app.WorkflowEngineTaskQueue,
	}
	// path := "https://ptsv2.com/t/epf3a-1621684148/post"
	path := "http://worldclockapi.com/api/json/est/now"
	we, err := c.ExecuteWorkflow(context.Background(), options, app.MegaWorkflow, path)
	if err != nil {
		log.Fatalln("unable to complete Workflow", err)
	}
	var greeting string
	err = we.Get(context.Background(), &greeting)
	if err != nil {
		log.Fatalln("unable to get Workflow result", err)
	}
	printResults(greeting, we.GetID(), we.GetRunID())
}

func printResults(greeting string, workflowID, runID string) {
	fmt.Printf("\nWorkflowID: %s RunID: %s\n", workflowID, runID)
	fmt.Printf("\n%s\n\n", greeting)
}
