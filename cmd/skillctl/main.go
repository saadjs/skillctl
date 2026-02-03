package main

import (
	"os"

	"github.com/saadjs/agent-skills/internal/cli"
)

func main() {
	cli.Execute(os.Args[1:])
}
