package main

import (
	"fmt"
	"json-config/shared"
)


func main() {
	type example struct {
		Hello string `json:"hello"`
	}

	jsonData := []byte(`
		{
			"abc": "123",
			"hello": "{{ abc }}",
			"search": "{{ testing|<titl }}",
			"testing": {
				"title": "Pie",
				"some_name": "Nick"
			},
			"arr": [
				"a", "b", "c"
			]
		}
	`)

	parser, err := shared.NewParser(&jsonData)
	if err != nil {
		panic(err)
	}

	var out example
	if err := parser.Parse(&out); err != nil {
		panic(err)
	}

	fmt.Printf("STRUCT: %+v\n", out)
}