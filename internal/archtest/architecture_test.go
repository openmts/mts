package archtest

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

const modulePath = "github.com/openmts/mts"

type packageInfo struct {
	ImportPath string
	Deps       []string
}

func TestArchitectureDependencies(t *testing.T) {
	packages := loadPackages(t)
	rules := []struct {
		name      string
		match     func(string) bool
		forbidden []string
	}{
		{
			name: "storage packages must not depend on engine",
			match: func(path string) bool {
				return strings.HasPrefix(path, modulePath+"/internal/memtable") ||
					strings.HasPrefix(path, modulePath+"/internal/sstable") ||
					strings.HasPrefix(path, modulePath+"/internal/wal") ||
					strings.HasPrefix(path, modulePath+"/internal/storagefs") ||
					strings.HasPrefix(path, modulePath+"/internal/storagecheck")
			},
			forbidden: []string{modulePath + "/internal/engine"},
		},
		{
			name: "query packages must not depend on engine",
			match: func(path string) bool {
				return strings.HasPrefix(path, modulePath+"/internal/query")
			},
			forbidden: []string{modulePath + "/internal/engine"},
		},
		{
			name: "model package must not depend on internal business packages",
			match: func(path string) bool {
				return path == modulePath+"/internal/model"
			},
			forbidden: []string{modulePath + "/internal/"},
		},
		{
			name: "root package must not depend on cmd packages",
			match: func(path string) bool {
				return path == modulePath
			},
			forbidden: []string{modulePath + "/cmd/"},
		},
	}
	for _, rule := range rules {
		t.Run(rule.name, func(t *testing.T) {
			for _, pkg := range packages {
				if !rule.match(pkg.ImportPath) {
					continue
				}
				for _, dep := range pkg.Deps {
					for _, forbidden := range rule.forbidden {
						if strings.HasPrefix(dep, forbidden) && dep != pkg.ImportPath {
							t.Fatalf("%s depends on forbidden package %s", pkg.ImportPath, dep)
						}
					}
				}
			}
		})
	}
}

func loadPackages(t *testing.T) []packageInfo {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "./...")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list failed: %v", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var packages []packageInfo
	for decoder.More() {
		var pkg packageInfo
		if err := decoder.Decode(&pkg); err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		if strings.HasPrefix(pkg.ImportPath, modulePath+"/cmd/") {
			continue
		}
		packages = append(packages, pkg)
	}
	return packages
}
