// Package architecture_test checks the import graph described in
// docs/architecture.md. It is a test-only package so that the rules run with
// every `go test ./...` and do not add a runtime dependency to the app.
package architecture_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const modulePath = "github.com/arfush/yonime"

// featureDirs are packages that implement a product feature. Platform code is
// allowed to use leaves such as log or version, but must not depend on them.
var featureDirs = []string{
	"animelib",
	"app",
	"browser",
	"catalog",
	"kodik",
	"library",
	"metadata",
	"mpv",
	"playback",
	"party",
	"store",
	"updates",
	"updater",
	"ui",
}

type packageInfo struct {
	imports map[string]bool
}

func TestImportCycles(t *testing.T) {
	packages := loadPackages(t)

	const (
		white = 0
		gray  = 1
		black = 2
	)
	state := make(map[string]int, len(packages))
	stack := make([]string, 0, len(packages))

	var visit func(string) bool
	visit = func(path string) bool {
		state[path] = gray
		stack = append(stack, path)
		for _, next := range sortedImports(packages[path]) {
			switch state[next] {
			case gray:
				start := 0
				for i, item := range stack {
					if item == next {
						start = i
						break
					}
				}
				cycle := append(append([]string{}, stack[start:]...), next)
				t.Errorf("import cycle: %s", strings.Join(cycle, " -> "))
				return false
			case white:
				if !visit(next) {
					return false
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[path] = black
		return true
	}

	for _, path := range sortedPackages(packages) {
		if state[path] == white {
			visit(path)
		}
	}
}

func TestImportBoundaries(t *testing.T) {
	packages := loadPackages(t)
	for _, from := range sortedPackages(packages) {
		for _, to := range sortedImports(packages[from]) {
			if reason := boundaryViolation(from, to); reason != "" {
				t.Errorf("%s imports %s: %s", short(from), short(to), reason)
			}
		}
	}
}

func boundaryViolation(from, to string) string {
	if under(to, modulePath+"/cmd") || under(to, modulePath+"/tools") {
		return "entry points (cmd, tools) must not be imported"
	}

	if under(to, modulePath+"/app") && !under(from, modulePath+"/cmd") {
		return "only cmd may import app"
	}

	fromUI := under(from, modulePath+"/ui")
	toUI := under(to, modulePath+"/ui")
	if toUI && !fromUI {
		if !under(from, modulePath+"/app") && !under(from, modulePath+"/cmd") {
			return "only app and cmd may import ui"
		}
	}
	if fromUI && under(to, modulePath+"/app") {
		return "ui must not import app: pass data and actions instead"
	}
	if fromUI && under(to, modulePath+"/store") {
		return "ui must not import store: data comes through app"
	}

	if isLeaf(from) {
		return "leaf packages must not import other project packages"
	}

	if under(from, modulePath+"/platform") {
		if under(to, modulePath+"/ui") || under(to, modulePath+"/app") || under(to, modulePath+"/cmd") {
			return "platform must not depend on ui, app or cmd"
		}
		for _, feature := range featureDirs {
			if under(to, modulePath+"/"+feature) {
				return "platform must not depend on features"
			}
		}
	}

	return ""
}

// isLeaf lists packages that form the bottom of the graph. They may use the
// standard library and third-party modules, but no other project package.
func isLeaf(path string) bool {
	switch path {
	case modulePath + "/catalog", modulePath + "/log", modulePath + "/token", modulePath + "/version":
		return true
	}
	return false
}

func loadPackages(t *testing.T) map[string]*packageInfo {
	t.Helper()

	root := repoRoot(t)
	packages := make(map[string]*packageInfo)
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".github", ".idea", ".vscode", "deploy", "dist", "docs", "installer", "node_modules", "third_party":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		dir := filepath.Dir(path)
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return err
		}
		pkgPath := modulePath
		if rel != "." {
			pkgPath = modulePath + "/" + filepath.ToSlash(rel)
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		info := packages[pkgPath]
		if info == nil {
			info = &packageInfo{imports: make(map[string]bool)}
			packages[pkgPath] = info
		}
		for _, spec := range file.Imports {
			imported := strings.Trim(spec.Path.Value, `"`)
			if under(imported, modulePath) {
				info.imports[imported] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repository: %v", err)
	}
	return packages
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate architecture test")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("go.mod not found in %s: %v", root, err)
	}
	return root
}

func under(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func short(path string) string {
	return strings.TrimPrefix(path, modulePath+"/")
}

func sortedPackages(packages map[string]*packageInfo) []string {
	paths := make([]string, 0, len(packages))
	for path := range packages {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func sortedImports(info *packageInfo) []string {
	if info == nil {
		return nil
	}
	imports := make([]string, 0, len(info.imports))
	for path := range info.imports {
		imports = append(imports, path)
	}
	sort.Strings(imports)
	return imports
}
