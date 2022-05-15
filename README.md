# gort
* go runtime type: get reflect.Type by name

# Usage
* lets you call functions in your binary with just the string of their name
``` go
	rt, err := gort.NewDwarfRT("")
	fmt.Printf("test call fmt.Printf\n")

	rets, err = rt.CallFunc("fmt.Printf", true, []reflect.Value{
		reflect.ValueOf("test call fmt.Printf:%d %s\n"),
		reflect.ValueOf(1234),
		reflect.ValueOf("hello"),
	})
```

* lets you get access to globals in your binary with just the string of their name
```go
	rt, err := gort.NewDwarfRT("")
	rGlobal, err := rt.FindGlobal("main.testGlobal")
```

* lets you get access to all of the `reflect.Types` in your binary of their name
    * Caveat: the types must be possible outputs to reflect.TypeOf(val) in your binary 
```go
	rt, err := gort.NewDwarfRT("")
	typ, err := rt.FindType("main.testStruct")
```

# Examples
* `go build -gcflags=all=-l examples/hello/hello.go`
* `./hello`
