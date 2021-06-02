package main

import (
	"fmt"
	"io/ioutil"

	"rogchap.com/v8go"
)

func main() {
	ctx, _ := v8go.NewContext()                             // creates a new V8 context with a new Isolate aka VM
	ctx.RunScript("const add = (a, b) => a + b", "math.js") // executes a script on the global context
	ctx.RunScript("const result1 = add(3, 4)", "main.js")   // any functions previously added to the context can be called
	val, _ := ctx.RunScript("result1", "value.js")          // return a value in JavaScript back to Go
	fmt.Printf("addition result: %s", val)

	data, err := ioutil.ReadFile("./z.js")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	val, _ = ctx.RunScript(string(data), "z.js")

	val, _ = ctx.RunScript("result", "value.js")
	fmt.Printf("matches: %s", val)

}
