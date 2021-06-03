package app

import (
	"regexp"
)

const WorkflowEngineTaskQueue = "WORKFLOW_ENGINE_TASK_QUEUE"

func UnEscapeStr(str string) string {
	m, _ := regexp.MatchString(`^"\\"([^\\]+)\\""$`, str)
	if m {
		return "\"" + str[3:len(str)-3] + "\""
	}
	// js var
	m, _ = regexp.MatchString(`^"([^"]+)"$`, str)
	if m {
		return str[1 : len(str)-1]
	}
	return str
}

func RemoveQuote(str string) string {
	m, _ := regexp.MatchString(`^"([^"]+)"$`, str)
	if m {
		return str[1 : len(str)-1]
	}
	return str
}
