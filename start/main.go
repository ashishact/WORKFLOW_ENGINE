package main

import (
	"context"
	"fmt"
	"log"

	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pborman/uuid"
	"go.temporal.io/sdk/client"

	"workflow_engine/app"
)

var temporalClient client.Client

func main() {

	// Create the client object just once per process
	c, err := client.NewClient(client.Options{})
	if err != nil {
		log.Fatalln("unable to create Temporal client", err)
	}
	temporalClient = c

	defer c.Close()

	// WEB SERVER

	PORT := 3001
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.GET("test", TestWorkflow)
	addr := ":" + strconv.Itoa(PORT)

	r.Run(addr) // listen and serve on 0.0.0.0:3001 (for windows "localhost:3001")
}

func printResults(greeting string, workflowID, runID string) {
	fmt.Printf("\nWorkflowID: %s RunID: %s\n", workflowID, runID)
	fmt.Printf("\n%s\n\n", greeting)
}

func TestWorkflow(c *gin.Context) {
	options := client.StartWorkflowOptions{
		ID:        "workflow-" + uuid.New(),
		TaskQueue: app.WorkflowEngineTaskQueue,
	}

	wfStr := c.Request.URL.Query().Get("workflow")
	if wfStr == "" {
		c.JSON(200, gin.H{
			"error": "Empty workflow",
		})
		return
	}

	var appWorkflow app.Workflow
	err := json.Unmarshal([]byte(wfStr), &appWorkflow)
	if err != nil {
		c.JSON(200, gin.H{
			"error": "Invalid JSON",
		})
		return
	}

	we, err := temporalClient.ExecuteWorkflow(context.Background(), options, app.SimpleDSLWorkflow, appWorkflow)
	if err != nil {
		log.Println("Unable to execute workflow", err)
	} else {
		log.Println("Started workflow", "WorkflowID", we.GetID(), "RunID", we.GetRunID())
	}

	// // path := "https://ptsv2.com/t/epf3a-1621684148/post"
	// path := "http://worldclockapi.com/api/json/est/now"
	// we, err := temporalClient.ExecuteWorkflow(context.Background(), options, app.MegaWorkflow, path)
	// if err != nil {
	// 	log.Println("unable to complete Workflow", err)
	// }

	if err != nil {
		c.JSON(200, gin.H{
			"error":      err,
			"workflowId": we.GetID(),
			"runId":      we.GetRunID(),
		})
	} else {
		c.JSON(200, gin.H{
			"workflowId": we.GetID(),
			"runId":      we.GetRunID(),
		})
	}

}
