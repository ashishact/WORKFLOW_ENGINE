package app

import (
	"io/ioutil"
	"log"
	"net/http"
	// "workflow_engine/app"
)

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
