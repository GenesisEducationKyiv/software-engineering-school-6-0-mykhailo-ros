// Package archtest enforces the layer boundaries documented in
// docs/architecture.md: every package under internal/ may only depend on
// internal/domain (plus itself); the composition root (cmd) is exempt and
// may wire anything together. notification-service has no separate
// "service" layer — scheduler plays that role (ticker-driven orchestration
// in place of an HTTP handler).
package archtest

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const modulePrefix = "notification-service/"

// allowedTargets maps each internal/ layer to the other internal/ layers it
// may import. Every layer implicitly allows importing itself (not listed).
var allowedTargets = map[string]map[string]bool{
	"domain":    {},
	"config":    {"domain": true},
	"cache":     {"domain": true},
	"github":    {"domain": true},
	"mailer":    {"domain": true},
	"client":    {"domain": true},
	"events":    {"domain": true},
	"scheduler": {"domain": true},
}

type goListPkg struct {
	ImportPath string   `json:"ImportPath"`
	Imports    []string `json:"Imports"`
}

// layerOf returns the internal/ layer name for a module-local import path,
// and whether it lives under internal/ at all (packages like cmd, or
// external packages, are not subject to the layer rules).
func layerOf(importPath string) (layer string, isInternal bool) {
	rest, ok := strings.CutPrefix(importPath, modulePrefix)
	if !ok {
		return "", false
	}
	internalRest, ok := strings.CutPrefix(rest, "internal/")
	if !ok {
		return "", false
	}
	return strings.SplitN(internalRest, "/", 2)[0], true
}

func TestLayerDependencies(t *testing.T) {
	modRoot, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("go env GOMOD: %v", err)
	}

	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = filepath.Dir(strings.TrimSpace(string(modRoot)))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -json ./... (from %s): %v", cmd.Dir, err)
	}

	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var pkg goListPkg
		if err := dec.Decode(&pkg); err != nil {
			t.Fatalf("decode go list output: %v", err)
		}

		sourceLayer, isInternal := layerOf(pkg.ImportPath)
		if !isInternal || sourceLayer == "archtest" {
			continue // cmd (composition root) and this test itself are exempt
		}

		allowed, known := allowedTargets[sourceLayer]
		if !known {
			t.Fatalf("package %q has unrecognized layer %q — add it to allowedTargets in architecture_test.go", pkg.ImportPath, sourceLayer)
		}

		for _, imp := range pkg.Imports {
			targetLayer, targetInternal := layerOf(imp)
			if !targetInternal || targetLayer == sourceLayer {
				continue
			}
			if !allowed[targetLayer] {
				t.Errorf("layer violation: %q (layer %q) must not import %q (layer %q) — see docs/architecture.md",
					pkg.ImportPath, sourceLayer, imp, targetLayer)
			}
		}
	}
}
