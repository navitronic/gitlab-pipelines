package main

import (
	"context"
	"fmt"
	"os"

	"github.com/navitronic/gitlab-builds/internal/glab"
)

func main() {
	ctx := context.Background()
	client := glab.New()

	user, err := client.CurrentUser(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Authenticated as: %s (%s)\n", user.Name, user.Username)
}
