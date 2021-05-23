package app

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

func MegaWorkflow(ctx workflow.Context, path string) (string, error) {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 5,
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	var result string
	err := workflow.ExecuteActivity(ctx, CallHttp, path).Get(ctx, &result)
	return result, err
}
