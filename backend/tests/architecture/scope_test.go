package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// A small regression gate, not a new tooling framework. Explicit authorization
// and review are required to change this boundary, not simply to silence it.
func TestRetiredToolingStaysOutsideApplication(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, path := range []string{"internal/localprofile", "internal/localcheck", "internal/localpostgres", "internal/localbootstrap", "internal/localbudget", "cmd/local-check"} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Errorf("retired path must not return: %s (stat: %v)", path, err)
		}
	}
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, spec := range file.Imports {
				name, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					return err
				}
				if strings.Contains(name, "/archive/") || strings.HasPrefix(name, "bidora/backend/internal/local") {
					t.Errorf("application must not import retired tooling: %s: %s", path, name)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
