package app

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	"github.com/robertkrimen/otto"
	"go.temporal.io/sdk/workflow"
	"rogchap.com/v8go"
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

	MatchT struct {
		On         interface{}
		Conditions []string
	}
	SwitchT struct {
		Condition string
		Next      string
	}

	// Each step/activity is a task that's individually executed by the engine in series
	Step struct {
		Name      string
		Call      string
		Args      map[string]interface{}
		Variables map[string]interface{}
		Result    string
		Return    string
		Assign    map[string]interface{}
		Error     string
		Switch    json.RawMessage
		Match     json.RawMessage
		Next      string
		Children  []*Step
	}

	// Root workflow type => This is where the JSON get's converted to
	WF struct {
		Name       string
		Variables  map[string]interface{}
		Error      string
		Steps      []*Step // JSON object
		Activities []*Step // Will be ordered: depth first from root => end
		Timeout    int
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
var Z_SRC = ""

// Recurssivly insert steps => Depth Firts
func (wf *WF) insertSteps(current *Step) {
	if current.Args == nil {
		current.Args = make(map[string]interface{})
	}

	if current.Variables == nil {
		current.Variables = make(map[string]interface{})
	}

	if current.Assign == nil {
		current.Assign = make(map[string]interface{})
	}

	if current.Children == nil {
		current.Children = []*Step{}
	}

	current.Result = strings.ReplaceAll(current.Result, " ", "_") // var can't be with space

	// @todo: there are other invalid veriables
	m, _ := regexp.MatchString("^\\d+", current.Result)
	if m {
		current.Result = "Invalid_" + current.Result
	}

	wf.Activities = append(wf.Activities, current)
	for _, s := range current.Children {
		wf.insertSteps(s)
	}
}

// Activities is an ordered tree structure expanded into an Array
func (wf *WF) createActivitiesFromSteps() {
	for _, s := range wf.Steps {
		wf.insertSteps(s)
	}
}

// Get index of a step by name
func (wf *WF) findStepIndex(name string) (int, error) {
	for i, a := range wf.Activities {
		if a.Name == name {
			return i, nil
		}
		// log.Println(a.Name, " != ", name)
	}
	return -1, errors.New("No steps found with name: " + name)
}

// Constructor function to create a new workflow
func NEW_WF(json_bytes []byte) (WF, error) {
	var wf WF
	err := json.Unmarshal(json_bytes, &wf)
	wf.createActivitiesFromSteps()

	if wf.Variables == nil {
		wf.Variables = make(map[string]interface{})
	}

	return wf, err
}

// Main workflow func executed by temporal
func WorkflowEngineMain(ctx workflow.Context, wf WF) (interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}

	if wf.Timeout > 0 && wf.Timeout < 60 {
		ao.StartToCloseTimeout = time.Duration(wf.Timeout) * time.Second
	}

	ctx = workflow.WithActivityOptions(ctx, ao)
	logger := workflow.GetLogger(ctx)

	// @todo: If JS required
	v8, _ := v8go.NewContext()

	v8.RunScript(Z_SRC, "z.js")

	code := "let ERRORS = [];"
	v8.RunScript(code, "init.js")

	// This for loop takes care of nested steps as well
	// because we are converting all nseted steps to an array with depth first order
	i := 0
	for i < len(wf.Activities) {
		step := wf.Activities[i]
		err := step.execute(ctx, v8)
		if err != nil {
			logger.Error("Workflow failed.", "Error", err)
			return "", err
		}

		// Replace all wf variables with the result of this step
		for k, v := range step.Variables {
			wf.Variables[k] = UnEscapeStr(v.(string))
			// wf.Variables[k] = v.(string)
		}

		if step.Return != "" {
			log.Println("FOUND RETURN. Ending workflow")
			break
		}

		// SWITCH
		var switches []SwitchT
		json.Unmarshal(step.Switch, &switches)
		if len(switches) > 0 {
			shouldJump := false
			for _, sw := range switches {
				val, _ := runJS(sw.Condition, v8, "switch")
				if val == "true" {
					log.Println(sw.Condition)
					log.Println("CONDITION TRUE: Next => " + sw.Next)

					nextI, err := wf.findStepIndex(sw.Next)
					if err == nil && nextI < len(wf.Activities) {
						i = nextI         // JUMP
						shouldJump = true // Catch after loop ends
						break             // inner loop
						// ; continue // Can't continuue here as ist's embeded loop
					}

				} else {
					log.Println(sw.Condition)
					log.Println("CONDITION FALSE")
				}
			}

			if shouldJump {
				// i must be already set to Next
				continue
			}

		}

		// NEXT
		if step.Next != "" {
			nextI, err := wf.findStepIndex(step.Next)
			if err == nil && nextI < len(wf.Activities) {
				i = nextI // JUMP
				continue
			}
		}

		i++
	}
	logger.Info("Workflow completed.")

	for k, v := range wf.Variables {
		log.Println("Variables => ", k, v)
	}

	jserr, _ := v8.RunScript("JSON.stringify(ERRORS)", "error.js")
	if jserr != nil {
		wf.Error = jserr.String()
	}

	returnValue := wf.Variables["return"]

	return returnValue, nil
}

// Each step is executed with ARGS/ASSIGN/RESULT/MATCH/RETURN
func (s *Step) execute(ctx workflow.Context, v8 *v8go.Context) error {
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

	// JS code to run in v8
	code := ""

	// Before Activity Parse Expression in inputs

	// ASSIGN
	for k, v := range s.Assign {
		bs, ok := json.Marshal(v)
		vs := string(bs)
		if ok == nil && vs != "" {
			vs = UnEscapeStr(vs)
			code = k + " = " + vs // "num: 1" => num = 1
			val, err := v8.RunScript(code, "assign.js")
			if err != nil {
				log.Println("ASSIGN: code ", code, err)
				code = "ERRORS.push(JSON.stringify(" + err.Error() + "));"
				v8.RunScript(code, "assign.error.js")
			} else {
				s.Assign[k] = val.String() // Assigned vars ready for activity
			}
		}
	}

	// ARGS
	for k, v := range s.Args {
		// IF STRING
		// ELSE
		vs, ok := v.(string)
		if ok && (vs != "") && IsJS(vs) {
			vs = UnEscapeStr(vs)
			code = "`" + vs + "`" // "abc${2+3}def" => "abc5def"
			val, err := v8.RunScript(code, "args.js")
			if err != nil {
				code = "ERRORS.push(JSON.stringify(" + err.Error() + "));"
				v8.RunScript(code, "args.error.js")
				log.Println("ARGS: code ERR: ", code, err)
			} else {
				s.Args[k] = val.String() // Inputs ready for activity
			}
		}
	}

	// IF No activity just do the JS task
	if ActivityName == "" {
		log.Println("STEP: " + s.Name + " NO Activity")
	} else {
		err := workflow.ExecuteActivity(ctx, ActivityName, s).Get(ctx, &result)
		if err != nil {
			return err
		}
	}

	// RESULT
	// In Result just put's the result of the activity
	if s.Result != "" {
		if IsJSON(result) {
			code = s.Result + " = " + result + "; "
		} else {
			code = s.Result + " = '" + result + "'; "
		}
		// code = s.Result + " = JSON.parse(" + result + ");" // This doesn't work why?
		_, err := v8.RunScript(code, "result.js")
		if err != nil {
			if err != nil {
				log.Println("RESULT: code => ", code, err)
				code = "ERRORS.push(JSON.stringify(" + err.Error() + "));"
				v8.RunScript(code, "result.error.js")
			}
		}

		// Just store the value as result of this step
		s.Variables[s.Result] = result
	}

	// MATCH (use z pattern matching library)
	var match MatchT
	json.Unmarshal(s.Match, &match)
	if len(match.Conditions) > 0 {
		on, err := json.Marshal(match.On)
		if err == nil {
			ons := string(on)
			if ons != "null" {
				ons = UnEscapeStr(ons) // on: currentTime.dayOfTheWeek
				code := "z.matches(" + ons + ")(" + strings.Join(match.Conditions, ", ") + ")"
				_, err = v8.RunScript(code, "match.js")
				log.Println("MATCH: ", code, err)
				if err != nil {
					log.Println("MATCH: ", code, err)
					code = "ERRORS.push(JSON.stringify(" + err.Error() + "));"
					v8.RunScript(code, "return.error.js")
				}
			}

		}
	}

	// RETURN
	if s.Return != "" {
		// CHECK IF STRING RETURN
		// @todo string is an exception
		// ELSE
		// code = "returnValue = JSON.stringify(" + s.Return + "); returnValue"
		// code = "returnValue = " + s.Return + "; returnValue"

		if IsJS(s.Return) {
			code = "`" + s.Return + "`" // "abc${2+3}def" => "abc5def"
		} else {
			code = "returnValue = JSON.stringify(" + s.Return + "); returnValue"
		}

		returnString, err := v8.RunScript(code, "return.js")
		if err != nil {
			log.Println("RETURN: code => ", code, err)
			code = "ERRORS.push(JSON.stringify(" + err.Error() + "));"
			v8.RunScript(code, "return.error.js")
		} else {
			returnValue := returnString.String()
			s.Variables["return"] = returnValue
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

// Run JS code in v8 and return result
func runJS(code string, v8 *v8go.Context, ref string) (string, error) {
	val, err := v8.RunScript(code, ref)
	log.Println("RUNJS:"+strings.ToUpper(ref)+" code => "+code, "err => ", err)
	if err != nil {
		code = "ERRORS.push(JSON.stringify(" + err.Error() + "));"
		v8.RunScript(code, ref+".error.js")
		return "", err
	}
	return val.String(), nil
}

func makeInput(argNames []string, argsMap map[string]string) []string {
	var args []string
	for _, arg := range argNames {
		args = append(args, argsMap[arg])
	}
	return args
}

func getStringFromJSON(raw json.RawMessage) string {
	bs, err := json.Marshal(raw)
	if err == nil {
		str := string(bs)
		if str != "null" || str != "" {
			str = UnEscapeStr(str)
			return str
		}
	}

	return ""
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

// Load the source code for the z.min.js
func loadZSrc() error {
	Z_SRC = "console.log('no z.min.js');"
	data, err := ioutil.ReadFile("./z.min.js")
	if err != nil {
		fmt.Println(err)
		log.Println("COULDN'T LOAD JS SOURCE")
		return err
	}
	log.Println("z.min.js loaded")
	Z_SRC = string(data)
	return nil
}

func InitWorkflowGlobals() {
	loadZSrc()
}
