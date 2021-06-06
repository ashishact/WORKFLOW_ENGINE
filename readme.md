WORKFLOW ENGINE 
===============

`A Google step function clone written in Go and Typescript`

This project is based on temporal.io Go SDK. https://github.com/temporalio/temporal


It has two parts

1. The worker app
2. The initiator

## Workflow worker
The workflow worker takes a pending task from the queue and completes it within the specified constraints

## Workflow initialtor 
Takes a json/yaml file and generates a task description which the worker will be able to perform

## Prerequisite
The temporal server is required for the next steps to work
see more @ https://github.com/temporalio/docker-compose/blob/main/README.md


## Build and run
```bash

    go build -o bin/worker-server worker/main.go
    go build -o bin/runtime-server start/main.go

    [Linux]
    GOOS=linux GOARCH=amd64 go build -o bin/worker-server-linux worker/main.go
    GOOS=linux GOARCH=amd64 go build -o bin/runtime-server-linux start/main.go
```

@note: z.min.js file is required to be at the working directory. It will be loaded at runtime for value based pattern matching. 

