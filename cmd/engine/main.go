// Command engine runs the Axyra core engine.
package main

import (
	"fmt"
	"os"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	fmt.Fprintf(os.Stdout, "axyra core-engine %s\n", version)
}
