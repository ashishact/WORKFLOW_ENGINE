package app

import (
	"log"
	"time"

	"encoding/json"

	"github.com/robertkrimen/otto"
	"go.temporal.io/sdk/workflow"
)

type (
	Args_Http struct {
		Url     string
		Method  string
		Headers map[string]string
		Body    map[string]string
		Query   map[string]string
		Auth    string
		Timeout int
	}

	Args_Sleep struct {
		Seconds int
	}

	Step struct {
		Name      string
		Call      string
		Args      map[string]interface{}
		Variables map[string]interface{}
		Result    string
		Return    string
		Assign    map[string]interface{}
		Next      string
		Children  []*Step
	}

	WF struct {
		Name       string
		Variables  map[string]interface{}
		Steps      []*Step // JSON object
		Activities []*Step // Will be ordered: depth first from root => end
	}

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

// Recurssivly insert steps => Depth Firts
func (wf *WF) insertSteps(current *Step) {
	if current.Variables == nil {
		current.Variables = make(map[string]interface{})
	}
	if current.Assign == nil {
		current.Assign = make(map[string]interface{})
	}
	if current.Assign == nil {
		current.Children = []*Step{}
	}

	wf.Activities = append(wf.Activities, current)
	for _, s := range current.Children {
		wf.insertSteps(s)
	}
}
func (wf *WF) createActivitiesFromSteps() {
	if wf.Variables == nil {
		wf.Variables = make(map[string]interface{})
	}

	for _, s := range wf.Steps {
		wf.insertSteps(s)
	}
}

func NEW_WF(name string, json_str string) (WF, error) {
	var wf WF
	err := json.Unmarshal([]byte(json_str), &wf)
	wf.createActivitiesFromSteps()

	return wf, err
}

func WorkflowEngineMain(ctx workflow.Context, wf WF) (string, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	logger := workflow.GetLogger(ctx)

	// @todo: If JS required
	jsvm := otto.New()

	// This for loop takes care of nested steps as well
	// because we are converting all nseted steps to an array with depth first order
	i := 0
	for i < len(wf.Activities) {
		step := wf.Activities[i]
		err := step.execute(ctx, jsvm)
		if err != nil {
			logger.Error("Workflow failed.", "Error", err)
			return "", err
		}

		// Replace all wf variables with the result of this step
		for k, v := range step.Variables {
			wf.Variables[k] = v
		}

		// Check conditional jump

		if step.Return != "" {
			log.Println("FOUND RETURN. Ending workflow")
			break
		}

		i++
	}
	logger.Info("Workflow completed.")

	for k, v := range wf.Variables {
		log.Println(k, v)
	}

	rv := wf.Variables["return"]
	returnValue, ok := rv.(string)
	if !ok {
		returnValue = ""
	}

	return returnValue, nil
}

func (s *Step) execute(ctx workflow.Context, jsvm *otto.Otto) error {
	var result string
	ActivityName := ""
	switch s.Call {
	case "sleep":
		ActivityName = "Sleep"
		break
	case "http.get":
		ActivityName = "CallHttp"
		break
	case "noops":
		ActivityName = "NopActivity"
		break
	default:
		ActivityName = ""
	}

	// IF No activity just do the JS task
	if ActivityName == "" {
		log.Println("Step: " + s.Name + " No activity")
	} else {
		err := workflow.ExecuteActivity(ctx, ActivityName, s).Get(ctx, &result)
		if err != nil {
			return err
		}
	}

	code := ""

	// Assign
	// for k, v :=  range s.Assign{
	// 	str, ok = v.(string)
	// 	code+= k + " = " +
	// }
	if s.Result != "" {
		s.Variables[s.Result] = result
		if IsJSON(result) {
			code += s.Result + " = " + result + "; "
		} else {
			code += s.Result + " = '" + result + "'; "
		}
	}

	if s.Return != "" {
		code += "returnValue = " + s.Return + "; returnValue"
	}

	if code != "" {
		log.Println("code: ", code)
		v, err := jsvm.Run(code)
		if err != nil {
			log.Println("JS error:", err)
		} else {
			if s.Return != "" {
				s.Variables["return"], _ = v.ToString()
			}
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

func IsJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}
