// Command engine runs the Axyra core engine.
package main

import "fmt"

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	fmt.Println("axyra core-engine", version)
}
