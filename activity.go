package app

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	// "workflow_engine/app"

	"go.temporal.io/sdk/activity"
)

type ActivityType struct {
}

func (a *ActivityType) NopActivity(ctx context.Context, input []string) (string, error) {
	name := activity.GetInfo(ctx).ActivityType.Name
	fmt.Printf("Run %s with input %v \n", name, input)
	return "Result_" + name, nil
}

func (a *ActivityType) CallHttp(ctx context.Context, input []string, arguments []string) (string, error) {
	name := activity.GetInfo(ctx).ActivityType.Name
	fmt.Printf("Run %s with input %v \r\n", name, input)

	if len(arguments) == 0 {
		return "", errors.New("URL was not provided for CallHttp")
	}

	url := arguments[0]

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

func (a *ActivityType) Sleep(ctx context.Context, input []string, arguments []string) error {
	if len(arguments) == 0 {
		return nil
	}

	duration, err := strconv.Atoi(arguments[0])
	if err != nil {
		return errors.New("Sleep: Not a valid duration. Must be a number (seconds)")
	}

	log.Println("Sleeping Start")
	time.Sleep(time.Duration(duration) * time.Second)
	log.Println("Sleeping Done")
	return nil
}
