// Command yo-miqt is the MIQT/Qt6 shell of yo. It mirrors the Gio chrome and
// exposes the snapshot mode used to verify it.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	qt "github.com/mappu/miqt/qt6"

	"github.com/arfush/yo/catalog"
	"github.com/arfush/yo/version"
	"yo-miqt/internal/shell"
	"yo-miqt/internal/snapshot"
)

func main() {
	snapshotPath := flag.String("snapshot", "", "render the shell offscreen to a PNG and exit")
	grabPath := flag.String("grab", "", "grab the shown window to a PNG and exit")
	siteName := flag.String("site", string(catalog.SiteYoanime), "active site: yoanime, yomanga or yoranobe")
	menu := flag.Bool("menu", false, "open the site menu in the snapshot")
	hover := flag.String("hover", "", "hovered region in the snapshot: site, minimize, maximize, close, footer")
	scale := flag.Int("scale", 1, "snapshot scale factor")
	size := flag.String("size", "840x550", "window size in logical pixels, WxH")
	flag.Parse()

	site := parseSite(*siteName)
	width, height := parseSize(*size)
	hoverRegion := parseRegion(*hover)

	capture := *snapshotPath != "" && *grabPath == ""
	if capture {
		os.Setenv("QT_QPA_PLATFORM", "offscreen")
		os.Setenv("QT_SCALE_FACTOR", "1")
	}
	qt.NewQApplication([]string{os.Args[0]})

	router := shell.NewRouter()
	router.Register(shell.ScreenHome, shell.NewHome())
	router.Show(shell.ScreenHome)

	chrome := shell.New(shell.Options{
		Site:    site,
		Version: "v" + version.Ver,
		Screen:  router,
	})
	chrome.Resize(width, height)

	if capture {
		chrome.SetState(shell.State{Site: site, Menu: *menu, Hover: hoverRegion})
		shot, err := snapshot.Capture(chrome.Widget(), snapshot.Options{Scale: *scale})
		if err != nil {
			fail("capture", err)
		}
		if err := snapshot.Save(*snapshotPath, shot); err != nil {
			fail("save", err)
		}
		fmt.Printf("wrote %s %dx%d scale=%d\n", *snapshotPath, shot.Bounds().Dx(), shot.Bounds().Dy(), *scale)
		return
	}

	if *grabPath != "" {
		chrome.Show()
		for i := 0; i < 6; i++ {
			qt.QCoreApplication_ProcessEvents2(qt.QEventLoop__AllEvents, 50)
			time.Sleep(50 * time.Millisecond)
		}
		shot, err := snapshot.CaptureScreen(chrome.Widget())
		if err != nil {
			fail("grab", err)
		}
		if err := snapshot.Save(*grabPath, shot); err != nil {
			fail("save", err)
		}
		fmt.Printf("wrote %s %dx%d\n", *grabPath, shot.Bounds().Dx(), shot.Bounds().Dy())
		chrome.Close()
		return
	}

	chrome.Show()
	qt.QApplication_Exec()
}

// parseSite validates the -site flag.
func parseSite(name string) catalog.Site {
	for _, site := range catalog.Sites() {
		if string(site) == name {
			return site
		}
	}
	fail("site", fmt.Errorf("unknown site %q", name))
	return ""
}

// parseSize reads the WxH -size flag.
func parseSize(value string) (int, int) {
	var width, height int
	if _, err := fmt.Sscanf(value, "%dx%d", &width, &height); err != nil || width <= 0 || height <= 0 {
		fail("size", fmt.Errorf("want WxH, got %q", value))
	}
	return width, height
}

// parseRegion reads the -hover flag.
func parseRegion(name string) shell.Region {
	switch strings.ToLower(name) {
	case "":
		return shell.RegionNone
	case "site":
		return shell.RegionSite
	case "minimize":
		return shell.RegionMinimize
	case "maximize":
		return shell.RegionMaximize
	case "close":
		return shell.RegionClose
	case "footer":
		return shell.RegionFooter
	}
	fail("hover", fmt.Errorf("unknown region %q", name))
	return shell.RegionNone
}

// fail prints one error and exits with a non-zero status.
func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "yo-miqt: %s: %v\n", what, err)
	os.Exit(1)
}
