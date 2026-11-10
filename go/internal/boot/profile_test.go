package boot_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"git.glkj.example.com/glkj/harness/go/internal/boot"
)

func configValue(node *yaml.Node, key string) (any, bool) {
	if node == nil {
		return nil, false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			var value any
			if err := node.Content[i+1].Decode(&value); err != nil {
				return nil, false
			}
			return value, true
		}
	}
	return nil, false
}

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestApplyInsertsThenPatchesByID(t *testing.T) {
	dir := t.TempDir()
	base := writeFile(t, dir, "base.yml", `
- insert:
    - id: session
      name: '@deepseek-ai/dsh-session'
    - id: tools
      name: '@deepseek-ai/dsh-tools'
`)
	overlay := writeFile(t, dir, "overlay.yml", `
- id: tools
  name: '@deepseek-ai/dsh-tools'
  config:
    parallel: 4
- insert:
    - id: shell
      name: '@deepseek-ai/dsh-shell'
`)

	layers := []boot.Layer{}
	for _, path := range []string{base, overlay} {
		layer, err := boot.LoadPatch(path)
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		layers = append(layers, layer)
	}

	tree := boot.NewTree()
	for _, layer := range layers {
		if err := tree.Apply(layer); err != nil {
			t.Fatalf("apply: %v", err)
		}
	}
	if len(tree.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(tree.Rows))
	}
	if tree.Rows[1].ID != "tools" {
		t.Fatalf("patched row = %+v", tree.Rows[1])
	}
	parallel, ok := configValue(tree.Rows[1].Config, "parallel")
	if !ok || parallel != 4 {
		t.Fatalf("parallel = %v, %v", parallel, ok)
	}
}

func TestPatchOfUnknownRowWarnsAndIsSkipped(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "patch.yml", `
- id: missing
  name: '@deepseek-ai/dsh-missing'
`)
	layer, err := boot.LoadPatch(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	tree := boot.NewTree()
	if err := tree.Apply(layer); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(tree.Rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(tree.Rows))
	}
	warnings := tree.Warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], `entry "missing" not found`) {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestDumpGroupsRowsByLayer(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "base.yml", `
- insert:
    - id: session
      name: '@deepseek-ai/dsh-session'
`)
	layer, err := boot.LoadPatch(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	tree := boot.NewTree()
	if err := tree.Apply(layer); err != nil {
		t.Fatalf("apply: %v", err)
	}
	dump, err := tree.Dump()
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	if !strings.Contains(dump, "# == "+path) || !strings.Contains(dump, "id: session") {
		t.Fatalf("dump =\n%s", dump)
	}
}
