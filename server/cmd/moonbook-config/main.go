package main

import (
	"fmt"
	"os"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/runtimeconfig"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: moonbook-config render <template> <output> | dsn")
		return 2
	}
	switch os.Args[1] {
	case "render":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: moonbook-config render <template> <output>")
			return 2
		}
		if err := runtimeconfig.Render(os.Args[2], os.Args[3], os.LookupEnv); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	case "dsn":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: moonbook-config dsn")
			return 2
		}
		dsn, err := runtimeconfig.DatabaseDSN(os.LookupEnv)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(dsn)
	default:
		fmt.Fprintf(os.Stderr, "unsupported action %q\n", os.Args[1])
		return 2
	}
	return 0
}
