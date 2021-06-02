package app

import (
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	"github.com/robertkrimen/otto"
	"go.temporal.io/sdk/workflow"
)

const (
	TYPE_NAME_INTEGER = iota
	TYPE_NAME_FLOAT
	TYPE_NAME_BOOLEAN
	TYPE_NAME_JSON
	TYPE_NAME_STRING
)

type (
	Var_Type struct {
		integer int64
		float   float64
		boolean bool
		json    string

		str      string
		typeName int
	}

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
		Error     string
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

var R_IS_JS, _ = regexp.Compile("\\$\\{[^\\}]+\\}")

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

	var result_var Var_Type
	result_var.str = result
	findType(&result_var)

	// Assign?
	// for ak, av := range s.Assign{

	// }

	// In Result just put's the result of the activity
	if s.Result != "" {
		if IsJSON(result) {
			code += s.Result + " = " + result + "; "
		} else {
			code += s.Result + " = '" + result + "'; "
		}
		s.Variables[s.Result] = result
	}

	// AT END
	if s.Return != "" {
		// Remove if any JS expression
		s.Return = strings.TrimSpace(s.Return)
		if s.Return[0:2] == "${" && s.Return[len(s.Return)-1:] == "}" {
			log.Println("Found JS expression")
			s.Return = s.Return[2 : len(s.Return)-1]
		}
		code += "returnValue = " + s.Return + "; returnValue"
	}

	if code != "" {
		log.Println("code: ", code)
		v, err := jsvm.Run(code)
		if err != nil {
			s.Error = "JS Error: " + err.Error()
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

// Returns true if this is a JS expression
func IsJS(s string) bool {
	return R_IS_JS.MatchString(s)
}
func IsJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}

func findType(var_type *Var_Type) {
	i, e := strconv.ParseInt(var_type.str, 10, 64)
	if e == nil {
		var_type.integer = i
		var_type.typeName = TYPE_NAME_INTEGER
		return
	}

	f, e := strconv.ParseFloat(var_type.str, 10)
	if e == nil {
		var_type.float = f
		var_type.typeName = TYPE_NAME_FLOAT
		return
	}

	if var_type.str == "true" || var_type.str == "True" {
		var_type.boolean = true
		var_type.typeName = TYPE_NAME_BOOLEAN
		var_type.str = "true"
		return
	}

	if var_type.str == "false" || var_type.str == "False" {
		var_type.boolean = false
		var_type.typeName = TYPE_NAME_BOOLEAN
		var_type.str = "false"
		return
	}

	if IsJSON(var_type.str) {
		var_type.json = var_type.str
		var_type.typeName = TYPE_NAME_JSON
		return
	}

	var_type.typeName = TYPE_NAME_STRING
	return
}
