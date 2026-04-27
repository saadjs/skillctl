package main

import (
	"os"

	"github.com/saadjs/skillctl/internal/cli"
)

func main() {
	cli.Execute(os.Args[1:])
}
