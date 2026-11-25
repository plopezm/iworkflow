//go:build ignore

// Generates the Windows resource objects (rsrc_windows_<arch>.syso) that embed the application icon in
// iworkflow-desktop.exe. Run from the repository root after changing the icon:
//
//	go run scripts/winres/main.go
//
// Resource ID 3 is the one Wails looks up for the window icon; it is also what Explorer and shortcuts show.
package main

import (
	"fmt"
	"os"

	"github.com/tc-hib/winres"
)

const (
	iconPath = "cmd/iworkflow-desktop/build/windows/icon.ico"
	outDir   = "cmd/iworkflow-desktop"
)

func run() error {
	f, err := os.Open(iconPath)
	if err != nil {
		return fmt.Errorf("opening icon: %w", err)
	}
	defer f.Close()
	icon, err := winres.LoadICO(f)
	if err != nil {
		return fmt.Errorf("reading icon: %w", err)
	}
	rs := &winres.ResourceSet{}
	if err = rs.SetIcon(winres.ID(3), icon); err != nil {
		return fmt.Errorf("setting icon: %w", err)
	}
	for arch, suffix := range map[winres.Arch]string{winres.ArchAMD64: "amd64", winres.ArchARM64: "arm64"} {
		out, err := os.Create(fmt.Sprintf("%s/rsrc_windows_%s.syso", outDir, suffix))
		if err != nil {
			return fmt.Errorf("creating object: %w", err)
		}
		err = rs.WriteObject(out, arch)
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return fmt.Errorf("writing %s object: %w", suffix, err)
		}
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
