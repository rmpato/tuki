// Command tuki is a tiny terminal companion that remembers the things you
// need to do.
package main

import (
	"context"
	"os"

	"github.com/rmpato/tuki/internal/cli"
)

func main() {
	if err := cli.Execute(context.Background()); err != nil {
		os.Exit(1)
	}
}
