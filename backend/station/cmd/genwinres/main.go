// Command genwinres generates the Windows resource object linked into the
// agent: the DriverGo icon and the version block.
//
// Why generate rather than commit the .syso: a .syso is an opaque object file
// nobody can review in a diff, and it would silently drift from
// build/drivergo.ico the moment the logo changed. This keeps the icon itself
// as the source of truth and rebuilds the resource on every image build, the
// same argument backend/Dockerfile already makes for cross-compiling the agent
// instead of committing the .exe.
//
// The version block is not decoration. An unsigned binary with no publisher,
// no product name and no version is exactly the shape SmartScreen and Chrome
// treat as anonymous; filling it in does not replace an Authenticode
// signature, but an executable that cannot even say what it is starts from a
// worse position.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

func main() {
	var (
		icon    = flag.String("icon", "build/drivergo.ico", "icon to embed")
		out     = flag.String("out", "cmd/avtotest-station/rsrc_windows_amd64.syso", "resource object to write")
		ver     = flag.String("version", "1.0.0", "product version, x.y.z")
		product = flag.String("product", "DriverGo Station", "product name shown in file properties")
		company = flag.String("company", "DriverGo", "publisher shown in file properties")
	)
	flag.Parse()

	f, err := os.Open(*icon)
	if err != nil {
		fail(err)
	}
	defer func() { _ = f.Close() }()

	ico, err := winres.LoadICO(f)
	if err != nil {
		fail(fmt.Errorf("load %s: %w", *icon, err))
	}

	rs := winres.ResourceSet{}
	// ID 1: Explorer shows the icon group with the lowest id, and the desktop
	// shortcut points at "<exe>,0", so the emblem must be first or both fall
	// back to the generic executable icon.
	rs.SetIcon(winres.ID(1), ico)

	vi := version.Info{}
	vi.SetFileVersion(*ver)
	vi.SetProductVersion(*ver)
	vi.Set(version.LangDefault, version.CompanyName, *company)
	vi.Set(version.LangDefault, version.ProductName, *product)
	vi.Set(version.LangDefault, version.FileDescription, *product)
	vi.Set(version.LangDefault, version.InternalName, "avtotest-station")
	vi.Set(version.LangDefault, version.OriginalFilename, "avtotest-station.exe")
	rs.SetVersionInfo(vi)

	w, err := os.Create(*out)
	if err != nil {
		fail(err)
	}
	if err := rs.WriteObject(w, winres.ArchAMD64); err != nil {
		_ = w.Close()
		fail(err)
	}
	if err := w.Close(); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s (icon %s, version %s)\n", *out, *icon, *ver)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "genwinres:", err)
	os.Exit(1)
}
