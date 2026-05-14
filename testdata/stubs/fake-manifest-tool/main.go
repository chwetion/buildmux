package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if os.Getenv("FAKE_MANIFEST_FAIL") == "1" {
		fmt.Fprintln(os.Stderr, "fake-manifest-tool: forced failure")
		os.Exit(1)
	}
	fmt.Printf("fake-manifest-tool args: %s\n", strings.Join(os.Args[1:], " "))
}
