package main

import (
	"fmt"
	"log"
	"reflect"

	"github.com/lsg2020/gort"
)

type testStruct struct {
	str string
}

func (p *testStruct) Print(format string, args ...interface{}) {
	log.Printf("in testStruct.Print %#v\n", p)
	log.Printf(format, args...)
}

var testGlobal = testStruct{
	str: "test global",
}

func main() {
	log.Printf("%#v\n", testGlobal)

	rt, err := gort.NewDwarfRT("")
	if err != nil {
		log.Fatalf("load dwarf err %s\n", err)
		return
	}

	// test find type
	typ, err := rt.FindType("main.testStruct")
	if err != nil {
		log.Fatalf("load type err %s\n", err)
		return
	}
	rTest := reflect.New(typ)
	log.Printf("new main.testStruct %#v", rTest.Interface())

	//  test global
	rGlobal, err := rt.FindGlobal("main.testGlobal")
	if err != nil {
		log.Fatalf("load global err %s\n", err)
		return
	}
	log.Printf("load  main.testGlobal %#v", rGlobal.Interface())

	// test func
	fmt.Printf("test call fmt.Printf\n")

	_, err = rt.CallFunc("fmt.Printf", true, []reflect.Value{
		reflect.ValueOf("test call fmt.Printf:%d %s\n"),
		reflect.ValueOf(1234),
		reflect.ValueOf("hello"),
	})
	if err != nil {
		log.Fatalf("test call err %s\n", err)
		return
	}

	_, err = rt.CallFunc("main.(*testStruct).Print", true, []reflect.Value{
		reflect.ValueOf(&testGlobal),
		reflect.ValueOf("test call method:%d %s\n"),
		reflect.ValueOf(1234),
		reflect.ValueOf("hello"),
	})
	if err != nil {
		log.Fatalf("test call err %s\n", err)
		return
	}

}
