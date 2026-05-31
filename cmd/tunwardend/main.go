package main

import (
	"context"
	"fmt"
	"os"

	"github.com/AidarKhusainov/tunwarden/internal/app/daemon"
)

func main() {
	if err := daemon.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tunwardend:", err)
		os.Exit(1)
	}
}
