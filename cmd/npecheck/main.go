package main

import (
	check "github.com/chenfeining/go-npecheck"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(check.Analyzer)
}
