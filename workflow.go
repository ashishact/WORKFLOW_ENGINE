package app

import (
	"log"
	"time"

	"encoding/json"

	"github.com/robertkrimen/otto"
	"go.temporal.io/sdk/workflow"
)

type (
	// Workflow is the type used to express the workflow definition. Variables are a map of valuables. Variables can be
	// used as input to Activity.
	Workflow struct {
		Variables map[string]string
		Root      Statement
	}

	// Statement is the building block of dsl workflow. A Statement can be a simple ActivityInvocation or it
	// could be a Sequence or Parallel.
	Statement struct {
		Activity *ActivityInvocation
		Sequence *Sequence
		Parallel *Parallel
	}

	// Sequence consist of a collection of Statements that runs in sequential.
	Sequence struct {
		Elements []*Statement
	}

	// Parallel can be a collection of Statements that runs in parallel.
	Parallel struct {
		Branches []*Statement
	}

	// ActivityInvocation is used to express invoking an Activity. The Arguments defined expected arguments as input to
	// the Activity, the result specify the name of variable that it will store the result as which can then be used as
	// arguments to subsequent ActivityInvocation.
	ActivityInvocation struct {
		Name      string
		Arguments []string
		Result    string
		Return    string
	}

	executable interface {
		execute(ctx workflow.Context, jsvm *otto.Otto, bindings map[string]string) error
	}
)

// SimpleDSLWorkflow workflow definition
func SimpleDSLWorkflow(ctx workflow.Context, appWorkflow Workflow) (string, error) {
	bindings := make(map[string]string)
	for k, v := range appWorkflow.Variables {
		bindings[k] = v
	}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	logger := workflow.GetLogger(ctx)

	// @todo: If JS required
	jsvm := otto.New()

	err := appWorkflow.Root.execute(ctx, jsvm, bindings)
	if err != nil {
		logger.Error("Workflow failed.", "Error", err)
		return "", err
	}
	logger.Info("Workflow completed.")

	result := bindings["return"]
	return result, err
}

func (b *Statement) execute(ctx workflow.Context, jsvm *otto.Otto, bindings map[string]string) error {
	if b.Parallel != nil {
		err := b.Parallel.execute(ctx, jsvm, bindings)
		if err != nil {
			return err
		}
	}
	if b.Sequence != nil {
		err := b.Sequence.execute(ctx, jsvm, bindings)
		if err != nil {
			return err
		}
	}
	if b.Activity != nil {
		err := b.Activity.execute(ctx, jsvm, bindings)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a ActivityInvocation) execute(ctx workflow.Context, jsvm *otto.Otto, bindings map[string]string) error {
	inputParam := makeInput(a.Arguments, bindings)
	var result string
	err := workflow.ExecuteActivity(ctx, a.Name, inputParam, a.Arguments).Get(ctx, &result)
	if err != nil {
		return err
	}

	code := ""
	if a.Result != "" {
		bindings[a.Result] = result
		if IsJSON(result) {
			code += a.Result + " = " + result + "; "
		} else {
			code += a.Result + " = '" + result + "'; "
		}

	}

	if a.Return != "" {
		code += "returnValue = " + a.Return + "; returnValue"
	}

	if code != "" {
		log.Println("code: ", code)
		v, err := jsvm.Run(code)
		if err != nil {
			log.Println("JS error:", err)
		} else {
			if a.Return != "" {
				bindings["return"], _ = v.ToString()
			}
		}
	}

	return nil
}

func (s Sequence) execute(ctx workflow.Context, jsvm *otto.Otto, bindings map[string]string) error {
	for _, a := range s.Elements {
		err := a.execute(ctx, jsvm, bindings)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p Parallel) execute(ctx workflow.Context, jsvm *otto.Otto, bindings map[string]string) error {
	//
	// You can use the context passed in to activity as a way to cancel the activity like standard GO way.
	// Cancelling a parent context will cancel all the derived contexts as well.
	//

	// In the parallel block, we want to execute all of them in parallel and wait for all of them.
	// if one activity fails then we want to cancel all the rest of them as well.
	childCtx, cancelHandler := workflow.WithCancel(ctx)
	selector := workflow.NewSelector(ctx)
	var activityErr error
	for _, s := range p.Branches {
		f := executeAsync(s, childCtx, jsvm, bindings)
		selector.AddFuture(f, func(f workflow.Future) {
			err := f.Get(ctx, nil)
			if err != nil {
				// cancel all pending activities
				cancelHandler()
				activityErr = err
			}
		})
	}

	for i := 0; i < len(p.Branches); i++ {
		selector.Select(ctx) // this will wait for one branch
		if activityErr != nil {
			return activityErr
		}
	}

	return nil
}

func executeAsync(exe executable, ctx workflow.Context, jsvm *otto.Otto, bindings map[string]string) workflow.Future {
	future, settable := workflow.NewFuture(ctx)
	workflow.Go(ctx, func(ctx workflow.Context) {
		err := exe.execute(ctx, jsvm, bindings)
		settable.Set(nil, err)
	})
	return future
}

func makeInput(argNames []string, argsMap map[string]string) []string {
	var args []string
	for _, arg := range argNames {
		args = append(args, argsMap[arg])
	}
	return args
}

/*
func MegaWorkflow(ctx workflow.Context, path string) (string, error) {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 20,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	var err error
	workflow.ExecuteActivity(ctx, Sleep, 5).Get(ctx, &err)

	var result string
	err = workflow.ExecuteActivity(ctx, CallHttp, path).Get(ctx, &result)
	return result, err
}
*/

func IsJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}
