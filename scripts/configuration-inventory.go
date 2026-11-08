// Configuration inventory scans names and references, never environment values.
// Run: go run ./scripts/configuration-inventory.go -root . -out /tmp/configuration-inventory.json
package main

import (
	"encoding/json"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Ref struct {
	Path  string `json:"path"`
	Line  int    `json:"line"`
	Scope string `json:"scope"`
}
type Row struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Source       Ref    `json:"source"`
	References   []Ref  `json:"references"`
	Status       string `json:"status"`
	Verification string `json:"verification"`
}
type Inventory struct {
	Scope           string   `json:"scope"`
	Parameters      []Row    `json:"parameters"`
	DynamicPatterns []string `json:"dynamic_patterns"`
	Exclusions      []string `json:"exclusions"`
}

var envPattern = regexp.MustCompile(`\b(?:BIDORA_[A-Z0-9_]+|NEXT_PUBLIC_[A-Z0-9_]+)\b`)
var standardEnv = regexp.MustCompile(`process\.env\.([A-Z][A-Z0-9_]*)`)
var environmentDeclaration = regexp.MustCompile(`^\s*(?:ENV\s+|#\s*)?([A-Z][A-Z0-9_]{2,})\s*[:=]`)
var nginxDirective = regexp.MustCompile(`^\s*([a-z][a-z0-9_]*)\s+[^{};]+;`)
var nextProperty = regexp.MustCompile(`^\s*([A-Za-z][A-Za-z0-9_]*)\s*:`)
var goEnv = regexp.MustCompile(`(?:Getenv|LookupEnv|getenv)\(\s*"([A-Z][A-Z0-9_]*)"`)

func scope(path string) string {
	if strings.HasPrefix(path, "archive/") {
		return "archive"
	}
	if strings.Contains(path, "/tests/") || strings.HasSuffix(path, "_test.go") {
		return "test"
	}
	if strings.HasPrefix(path, "docs/") || strings.HasSuffix(path, ".md") {
		return "documentation"
	}
	if strings.Contains(path, ".example") {
		return "example"
	}
	return "runtime"
}
func configType(name string) bool {
	for _, suffix := range []string{"Config", "Configuration", "Policy", "Limits", "Options", "Controls"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	switch name {
	case "Information", "Template", "Plan", "TermSnapshot", "TaxProfile":
		return true
	}
	return false
}
func main() {
	root := flag.String("root", ".", "repository root")
	out := flag.String("out", "", "JSON output path")
	flag.Parse()
	environments := map[string][]Ref{}
	selectors := map[string][]Ref{}
	rows := []Row{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(*root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, e := filepath.Rel(*root, path)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" || d.Name() == ".next" || d.Name() == ".cache" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".email-") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		allowed := strings.Contains(" .go .ts .tsx .js .mjs .py .sh .yml .yaml .json .toml .conf .md .example ", " "+ext+" ")
		if d.Name() == "Dockerfile" || strings.HasSuffix(d.Name(), ".Dockerfile") {
			allowed = true
		}
		if !allowed || strings.Contains(rel, "/persistence/sqlc/") || strings.Contains(rel, ".generated.") || strings.Contains(rel, "api/openapi/bidora.") || rel == "postman.json" || strings.HasPrefix(rel, "docs/audits/") {
			return nil
		}
		if strings.Contains(d.Name(), ".env") && !strings.HasSuffix(d.Name(), ".example") {
			return nil
		}
		raw, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		for i, line := range strings.Split(string(raw), "\n") {
			found := envPattern.FindAllString(line, -1)
			if strings.HasSuffix(rel, ".example") || strings.HasSuffix(rel, ".yml") || strings.HasSuffix(rel, "Dockerfile") {
				if match := environmentDeclaration.FindStringSubmatch(line); len(match) > 0 {
					found = append(found, match[1])
				}
			}
			var directive []string
			if ext == ".conf" {
				directive = nginxDirective.FindStringSubmatch(line)
			} else if strings.HasSuffix(rel, "next.config.ts") {
				directive = nextProperty.FindStringSubmatch(line)
			}
			if len(directive) > 0 {
				id := rel + "::" + directive[1]
				exists := false
				for j := range rows {
					if rows[j].ID == id {
						rows[j].References = append(rows[j].References, Ref{rel, i + 1, scope(rel)})
						exists = true
						break
					}
				}
				if !exists {
					rows = append(rows, Row{ID: id, Kind: "configuration file directive", Source: Ref{rel, i + 1, scope(rel)}, Status: "Partial", Verification: "Deployment/framework directive identified; not an administrator setting. Verify effective configuration in the target deployment."})
				}
			}
			for _, pattern := range []*regexp.Regexp{standardEnv, goEnv} {
				for _, match := range pattern.FindAllStringSubmatch(line, -1) {
					found = append(found, match[1])
				}
			}
			seen := map[string]bool{}
			for _, name := range found {
				if !seen[name] {
					environments[name] = append(environments[name], Ref{rel, i + 1, scope(rel)})
					seen[name] = true
				}
			}
		}
		if ext != ".go" || !strings.HasPrefix(rel, "backend/internal/") || scope(rel) != "runtime" {
			return nil
		}
		file, e := parser.ParseFile(fset, path, raw, parser.ParseComments)
		if e != nil {
			return e
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if s, ok := node.(*ast.SelectorExpr); ok {
				selectors[s.Sel.Name] = append(selectors[s.Sel.Name], Ref{rel, fset.Position(s.Pos()).Line, "runtime"})
			}
			return true
		})
		for _, decl := range file.Decls {
			general, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range general.Specs {
				t, ok := spec.(*ast.TypeSpec)
				if !ok || !configType(t.Name.Name) {
					continue
				}
				s, ok := t.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range s.Fields.List {
					for _, name := range field.Names {
						rows = append(rows, Row{ID: rel + "::" + t.Name.Name + "." + name.Name, Kind: "typed configuration field", Source: Ref{rel, fset.Position(name.Pos()).Line, "runtime"}, Status: "Partial", Verification: "Declaration and selector references inventoried; see audited family matrix for behavior-test evidence. References with the same field name are candidates, not type-resolution proof."})
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	for i := range rows {
		if rows[i].Kind == "typed configuration field" {
			parts := strings.Split(rows[i].ID, ".")
			rows[i].References = selectors[parts[len(parts)-1]]
		}
	}
	for name, refs := range environments {
		active := false
		for _, ref := range refs {
			if ref.Scope == "runtime" {
				active = true
			}
		}
		status, verification := "Unused", "No active runtime reference: test/example/documentation/archive input, not an administrator override."
		if active {
			status = "Partial"
			verification = "Source references inventoried; deployment-specific values, file contents and external dependencies were not read or activated. See family matrix."
		}
		rows = append(rows, Row{ID: name, Kind: "environment name", Source: refs[0], References: refs, Status: status, Verification: verification})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	inventory := Inventory{Scope: "Current source tree including uncommitted source; names/locations only. Static inventory is not runtime acceptance.", Parameters: rows, DynamicPatterns: []string{"BIDORA_TENANT_{N}_{ID,CLUSTER_ID,ASSIGNMENT_GENERATION,DATABASE_NAME,SECRET_REF,ENDPOINT_REF,REGION}", "BIDORA_{CONTROL,TENANT_{N}}_POSTGRES_{HOST,PORT,DATABASE,USER,PASSWORD,CA_FILE}", "BIDORA_MFA_KEY_{VERSION}_FILE"}, Exclusions: []string{"dependency/vendor/cache directories", "generated sqlc/API duplicates", "actual environment/secret files and environment values", "font/binary assets"}}
	encoded, e := json.MarshalIndent(inventory, "", "  ")
	if e != nil {
		panic(e)
	}
	encoded = append(encoded, '\n')
	if *out == "" {
		os.Stdout.Write(encoded)
	} else {
		if e = os.MkdirAll(filepath.Dir(*out), 0755); e != nil {
			panic(e)
		}
		if e = os.WriteFile(*out, encoded, 0644); e != nil {
			panic(e)
		}
	}
}
