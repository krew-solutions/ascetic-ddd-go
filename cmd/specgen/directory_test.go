package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The specifications of a directory: which package they are of, and in what
// order they come. Both used to depend on the order Go walks a map in.

// collected parses a directory of the files given and collects its
// specifications of Store.
func collected(t *testing.T, files map[string]string) (string, []SpecFunc, error) {
	t.Helper()
	dir := t.TempDir()
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, sourceFiles, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	return collect(fset, pkgs, "Store")
}

func names(specs []SpecFunc) string {
	found := make([]string, 0, len(specs))
	for _, s := range specs {
		found = append(found, s.Name)
	}
	return strings.Join(found, " ")
}

func TestTheGeneratedFileIsOfThePackageOfTheSpecifications(t *testing.T) {
	// A tool kept beside the package under a build tag is a package of the
	// directory to the parser, which does not read build tags. Whichever the
	// map gave last used to name the generated file: `package main` in a
	// package called shop, twelve runs out of twelve.
	pkg, specs, err := collected(t, map[string]string{
		"store.go": "package shop\n\ntype Store struct{ A int }\n\n//spec:sql\nfunc AlphaSpec(s Store) bool { return s.A > 1 }\n",
		"tool.go":  "//go:build ignore\n\npackage main\n\nfunc main() {}\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pkg != "shop" {
		t.Errorf("the package is %q", pkg)
	}
	if names(specs) != "AlphaSpec" {
		t.Errorf("found %s", names(specs))
	}
}

func TestSpecificationsOfTwoPackagesAreRefused(t *testing.T) {
	// One file is generated, of one package.
	_, _, err := collected(t, map[string]string{
		"a.go": "package shop\n\ntype Store struct{ A int }\n\n//spec:sql\nfunc AlphaSpec(s Store) bool { return s.A > 1 }\n",
		"b.go": "//go:build ignore\n\npackage main\n\ntype Store struct{ B int }\n\n//spec:sql\nfunc BetaSpec(s Store) bool { return s.B > 1 }\n",
	})
	if err == nil || !strings.Contains(err.Error(), "main") || !strings.Contains(err.Error(), "shop") {
		t.Fatalf("not refused as of two packages: %v", err)
	}
}

func TestSpecificationsComeInTheOrderOfTheirFiles(t *testing.T) {
	// The files of a package are a map: the order of the generated functions
	// changed from one run to the next.
	files := map[string]string{
		"c.go": "package shop\n\n//spec:sql\nfunc GammaSpec(s Store) bool { return s.A > 3 }\n",
		"a.go": "package shop\n\ntype Store struct{ A int }\n\n//spec:sql\nfunc AlphaSpec(s Store) bool { return s.A > 1 }\n\n//spec:sql\nfunc AlphaTwoSpec(s Store) bool { return s.A > 2 }\n",
		"b.go": "package shop\n\n//spec:sql\nfunc BetaSpec(s Store) bool { return s.A > 2 }\n",
	}
	for i := 0; i < 20; i++ {
		_, specs, err := collected(t, files)
		if err != nil {
			t.Fatal(err)
		}
		if got := names(specs); got != "AlphaSpec AlphaTwoSpec BetaSpec GammaSpec" {
			t.Fatalf("run %d: %s", i, got)
		}
	}
}

func TestTheMarkerIsTheLine(t *testing.T) {
	// `strings.Contains` marked a function whose comment merely mentioned the
	// marker, and one whose doc showed it in an example.
	_, specs, err := collected(t, map[string]string{
		"store.go": strings.Join([]string{
			"package shop",
			"",
			"type Store struct{ A int }",
			"",
			"// Marked checks A.",
			"//",
			"//spec:sql",
			"func Marked(s Store) bool { return s.A > 1 }",
			"",
			"// Mentioned is not a spec:sql function.",
			"func Mentioned(s Store) bool { return s.A > 1 }",
			"",
			"// Shown has the marker in an example:",
			"//",
			"//	//spec:sql",
			"//	func X(s Store) bool",
			"func Shown(s Store) bool { return s.A > 1 }",
			"",
			"//spec:sql   ",
			"func Trailing(s Store) bool { return s.A > 1 }",
			"",
		}, "\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(specs); got != "Marked Trailing" {
		t.Errorf("found %s", got)
	}
}

func TestTheMarkerWrittenWithASpaceIsSaidSo(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)
	_, specs, err := collected(t, map[string]string{
		"store.go": "package shop\n\ntype Store struct{ A int }\n\n// spec:sql\nfunc Spaced(s Store) bool { return s.A > 1 }\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 0 {
		t.Errorf("found %s", names(specs))
	}
	if !strings.Contains(logged.String(), "Spaced") || !strings.Contains(logged.String(), "//spec:sql") {
		t.Errorf("not said: %q", logged.String())
	}
}
