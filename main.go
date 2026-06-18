// Command rig is a single CLI for standing up, authenticating, navigating, and
// managing local coding projects.
package main

import (
	"os"

	"github.com/AndrewMast/rig/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
