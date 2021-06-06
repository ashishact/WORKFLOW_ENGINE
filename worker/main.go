package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"workflow_engine/app"
)

func main() {

	envNotFount := godotenv.Load()
	if envNotFount != nil {
		log.Println(".env not found")
	}

	app.InitWorkflowGlobals() // This will load the js file into memory

	// Create the client object just once per process
	option := client.Options{}
	if os.Getenv("HOSTPORT") != "" {
		option = client.Options{HostPort: os.Getenv("HOSTPORT")}
	}

	c, err := client.NewClient(option)
	if err != nil {
		log.Fatalln("unable to create Temporal client", err)
	}
	defer c.Close()
	// This worker hosts both Worker and Activity functions
	w := worker.New(c, app.WorkflowEngineTaskQueue, worker.Options{})

	w.RegisterWorkflow(app.WorkflowEngineMain)
	w.RegisterActivity(&app.ActivityType{})

	err = w.Run(nil) // Don't stop on error
	if err != nil {
		log.Fatalln("unable to start Worker", err)
	}
}
