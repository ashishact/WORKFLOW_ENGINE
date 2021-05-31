package main

import (
	"context"
	"log"

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
	PORT := 3007
	r := gin.Default()

	r.GET("/api/v1/run", TestWorkflow)
	addr := ":" + strconv.Itoa(PORT)

	r.Run(addr) // listen and serve on 0.0.0.0:3007 (for windows "localhost:3007")
}

func TestWorkflow(c *gin.Context) {
	options := client.StartWorkflowOptions{
		ID:        "workflow-" + uuid.New(),
		TaskQueue: app.WorkflowEngineTaskQueue,
	}

	wfStr := c.Request.URL.Query().Get("workflow")
	if wfStr == "" {
		c.JSON(200, gin.H{
			"status": "fail",
			"error":  "Empty workflow",
		})
		return
	}
	wf, err := app.NEW_WF("hello", wfStr)
	if err != nil {
		c.JSON(200, gin.H{
			"status": "fail",
			"error":  err.Error(),
		})
		return
	}

	we, err := temporalClient.ExecuteWorkflow(context.Background(), options, app.WorkflowEngineMain, wf)
	if err != nil {
		log.Println("Unable to execute workflow", err)
	} else {
		log.Println("Started workflow", "WorkflowID", we.GetID(), "RunID", we.GetRunID())
	}

	if err != nil {
		c.JSON(200, gin.H{
			"status":     "fail",
			"error":      err,
			"workflowId": we.GetID(),
			"runId":      we.GetRunID(),
		})
	} else {
		c.JSON(200, gin.H{
			"status":     "success",
			"workflowId": we.GetID(),
			"runId":      we.GetRunID(),
		})
	}
}
