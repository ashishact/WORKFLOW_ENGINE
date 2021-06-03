package main

import (
	"context"
	"log"
	"os"

	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/pborman/uuid"
	"go.temporal.io/sdk/client"

	"workflow_engine/app"
)

var temporalClient client.Client

func main() {
	envNotFount := godotenv.Load()
	if envNotFount != nil {
		log.Println(".env not found")
	}

	option := client.Options{}
	if os.Getenv("HOSTPORT") != "" {
		option = client.Options{HostPort: os.Getenv("HOSTPORT")}
	}

	// Create the client object just once per process
	c, err := client.NewClient(option)
	if err != nil {
		log.Fatalln("unable to create Temporal client", err)
	}
	temporalClient = c

	defer c.Close()

	// WEB SERVER
	PORT := 3007
	r := gin.Default()

	r.POST("/api/v1/run", TestWorkflow)
	addr := ":" + strconv.Itoa(PORT)

	r.Run(addr) // listen and serve on 0.0.0.0:3007 (for windows "localhost:3007")
}

func TestWorkflow(c *gin.Context) {
	options := client.StartWorkflowOptions{
		ID:        "workflow-" + uuid.New(),
		TaskQueue: app.WorkflowEngineTaskQueue,
	}

	body, err := c.GetRawData()
	if err != nil || len(body) < 2 {
		c.JSON(400, gin.H{
			"status": "fail",
			"error":  "Empty workflow",
		})
		return
	}
	wf, err := app.NEW_WF(body)
	if err != nil {
		c.JSON(500, gin.H{
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
		c.JSON(500, gin.H{
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
