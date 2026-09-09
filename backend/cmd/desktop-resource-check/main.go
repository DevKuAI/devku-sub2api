// desktop-resource-check validates a ZIP locally without publishing it.
package main

import (
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: desktop-resource-check resource.zip")
		os.Exit(2)
	}
	info, err := os.Stat(os.Args[1])
	if err != nil || info.Size() > service.DesktopResourceMaxBytes {
		fmt.Fprintln(os.Stderr, "cannot read package or package too large")
		os.Exit(1)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read package")
		os.Exit(1)
	}
	manifest, hash, err := service.ValidateDesktopResourcePackage(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid resource package")
		os.Exit(1)
	}
	fmt.Printf("Valid: %s v%s (%s), SHA256 %s\n", manifest.Key, manifest.Version, manifest.Platform, hash)
}
