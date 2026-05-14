package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if os.Getenv("FAKE_BUILDCTL_FAIL") == "1" {
		fmt.Fprintln(os.Stderr, "fake-buildctl: forced failure")
		os.Exit(1)
	}
	fmt.Printf("fake-buildctl args: %s\n", strings.Join(os.Args[1:], " "))
}
