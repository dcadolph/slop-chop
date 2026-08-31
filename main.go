// slop-chop finds and removes the tells of AI writing from text. The check command
// reports tells and gates CI, fix rewrites the text, and score rates it from 0 to 100.
// The engine behind every command is the importable sanitize package.
package main

import "github.com/dcadolph/slop-chop/cmd"

// main hands off to the command tree.
func main() {
	cmd.Execute()
}
