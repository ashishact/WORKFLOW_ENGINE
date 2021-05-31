package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	// "workflow_engine/app"

	"go.temporal.io/sdk/activity"
)

type ActivityType struct {
}

func (a *ActivityType) NopActivity(ctx context.Context, step *Step) (string, error) {
	name := activity.GetInfo(ctx).ActivityType.Name

	args, err := json.Marshal(step.Args)
	if err != nil {
		fmt.Printf("Run %s with args %v \n", name, args)
	}
	return "Result_" + name, nil
}

func (a *ActivityType) CallHttp(ctx context.Context, step *Step) (string, error) {
	name := activity.GetInfo(ctx).ActivityType.Name
	args, err := json.Marshal(step.Args)
	if err != nil {
		fmt.Printf("Run %s with args %v \n", name, args)
	}

	url := step.Args["url"].(string)

	if url == "" {
		return "", errors.New("URL was not provided for CallHttp")
	}

	res, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return "", err
	}
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Println(err)
		return "", err
	}
	result := string(body)

	return result, nil
}

func (a *ActivityType) Sleep(ctx context.Context, step *Step) error {
	name := activity.GetInfo(ctx).ActivityType.Name

	args, err := json.Marshal(step.Args)
	if err != nil {
		fmt.Printf("Run %s with args %v \n", name, args)
	}

	i := step.Args["seconds"]
	if i == nil {
		return errors.New("Sleep: No arguments. provide seconds")
	}

	seconds, ok := i.(float64)
	if !ok {
		return errors.New("Not valid seconds must be a number")
	}

	// duration, err := strconv.Atoi(seconds)
	// if err != nil {
	// 	return errors.New("Sleep: Not a valid duration. Must be a number (seconds)")
	// }

	duration := int(seconds)

	log.Println("Sleeping Start")
	time.Sleep(time.Duration(duration) * time.Second)
	log.Println("Sleeping Done")
	return nil
}
