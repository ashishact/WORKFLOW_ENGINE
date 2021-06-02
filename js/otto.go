package main

import (
	"fmt"
	"io/ioutil"

	"github.com/robertkrimen/otto"
)

func require(call otto.FunctionCall) otto.Value {
	file := call.Argument(0).String()
	fmt.Printf("requiring: %s\n", file)
	data, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	_, err = call.Otto.Run(string(data))
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	return otto.TrueValue()
}
func main1() {
	vm := otto.New()
	vm.Set("require", require)
	vm.Run(`
		require("z.js");
	`)
	vm.Run(`
    	abc = 2 + 2;
    	console.log("The value of abc is " + abc); // 4
	`)

}
