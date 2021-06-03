package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

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

	bs, err := json.Marshal(step.Args["seconds"])
	if err != nil {
		return errors.New("Sleep: No arguments. provide seconds")
	}

	str := UnEscapeStr(string(bs)) // Get 5 not "5"
	seconds, err := strconv.Atoi(str)
	if err != nil {
		return errors.New("Sleep: Not a valid seconds, must be a number: " + str)
	}

	duration := int(seconds)

	log.Println("Sleeping Start")
	time.Sleep(time.Duration(duration) * time.Second)
	log.Println("Sleeping Done")
	return nil
}

/*
h := json.RawMessage(`{"precomputed": true}`)

c := struct {
	Header *json.RawMessage `json:"header"`
	Body   string           `json:"body"`
}{Header: &h, Body: "Hello Gophers!"}

b, err := json.MarshalIndent(&c, "", "\t")
if err != nil {
	fmt.Println("error:", err)
}
*/
