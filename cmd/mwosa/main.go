package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ev3rlit/mwosa/cli"
)

func main() {
	if err := cli.NewRootCommand(cli.BuildInfo{}).ExecuteContext(context.Background()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
