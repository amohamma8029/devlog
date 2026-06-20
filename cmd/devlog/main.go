package main

import (
	"fmt"
	"os"
)

func main() {
	if err := Execute(); err != nil {
		fmt.Fprint(os.Stderr, renderCLIError(err))
		os.Exit(1)
	}
}
