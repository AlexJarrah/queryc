package main

import (
	"fmt"
	"os"

	"github.com/AlexJarrah/queryc/internal/app"
	"github.com/AlexJarrah/queryc/internal/cli"
)

func main() {
	opts, err := cli.Parse(os.Args[1:])
	if err != nil {
		if cli.IsHelp(err) {
			_, _ = fmt.Fprintln(os.Stdout, cli.Usage)
			return
		}

		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, cli.Usage)
		os.Exit(2)
	}

	if err := app.Run(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
