package main

import (
	"fmt"
	"os"

	"spacedb/parser"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: spacedb <sql>")
		os.Exit(2)
	}

	statement, err := parser.Parse(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%T: %v\n", statement, statement)
}
