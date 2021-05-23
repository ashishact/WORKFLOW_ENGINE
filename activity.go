package app

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	// "workflow_engine/app"

	"go.temporal.io/sdk/activity"
)

type SampleActivities struct {
}

func (a *SampleActivities) SampleActivity1(ctx context.Context, input []string) (string, error) {
	name := activity.GetInfo(ctx).ActivityType.Name
	fmt.Printf("Run %s with input %v \n", name, input)
	return "Result_" + name, nil
}

func (a *SampleActivities) CallHttp(ctx context.Context, input []string, arguments []string) (string, error) {
	name := activity.GetInfo(ctx).ActivityType.Name
	fmt.Printf("Run %s with input %v \r\n", name, input)

	if len(arguments) == 0 {
		return "", errors.New("Path was not provided")
	}
	for _, v := range arguments {
		log.Println(v)
	}
	path := arguments[0]

	res, err := http.Get(path)
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

/*
func CallHttp(path string) (string, error) {
	log.Println("Path: %s!", path)

	res, err := http.Get(path)
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

func Sleep(duration time.Duration) error {
	log.Println("Sleeping Start")
	time.Sleep(duration * time.Second)
	log.Println("Sleeping Done")
	return nil
}
*/
