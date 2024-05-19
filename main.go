package main

import (
	"github.com/prathoss/hw/cmd"
	"github.com/prathoss/hw/pkg"
)

func main() {
	pkg.SetupLogger()
	cmd.Execute()
}
