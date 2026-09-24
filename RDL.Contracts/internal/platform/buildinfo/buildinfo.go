// Package buildinfo expone la versión del CLI. Se fija al compilar:
//
//	go build -ldflags "-X bitbucket.org/rdl/contracts/internal/platform/buildinfo.Version=v0.1.0" ./cmd/contractsctl
package buildinfo

import "runtime/debug"

var Version = ""

// Current devuelve la versión fijada al compilar, la del módulo si se instaló con `go install ...@vX`, o "dev".
func Current() string {
	if Version != "" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}
