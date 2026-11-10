// Package boot composes the plugin tree from ordered profile layers, the Go
// counterpart of packages/boot/app-boot.
package boot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Row is one plugin row of the composed tree.
type Row struct {
	ID       string         `yaml:"id,omitempty"`
	Name     string         `yaml:"name,omitempty"`
	Inject   []string       `yaml:"inject,omitempty"`
	Config   map[string]any `yaml:"config,omitempty"`
	Disabled *Flag          `yaml:"disabled,omitempty"`

	origin string
}

// Flag is a boolean that may also be a Cordis expression (`!!js`), which the Go
// runtime keeps unevaluated until the plugin host can evaluate it.
type Flag struct {
	Value *bool
	Expr  string
}

// UnmarshalYAML accepts a literal boolean or a `!!js` expression.
func (f *Flag) UnmarshalYAML(node *yaml.Node) error {
	switch node.Tag {
	case "!!bool":
		var value bool
		if err := node.Decode(&value); err != nil {
			return err
		}
		f.Value = &value
		return nil
	case "!!js", "!js":
		f.Expr = node.Value
		return nil
	default:
		return fmt.Errorf("boot: disabled must be a boolean or a !!js expression, got %s", node.Tag)
	}
}

// MarshalYAML writes the original form back.
func (f Flag) MarshalYAML() (any, error) {
	if f.Value != nil {
		return *f.Value, nil
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!js", Style: yaml.DoubleQuotedStyle, Value: f.Expr}, nil
}

// Entry is one element of a patch file: either an insert block or a row that
// patches an existing row by id.
type Entry struct {
	Insert []Row `yaml:"insert,omitempty"`
	Row    `yaml:",inline"`
}

// Layer is one ordered layer applied to the empty entry list.
type Layer struct {
	Label   string
	Entries []Entry
}

// LoadPatch reads a cordis patch file (a YAML list of insert blocks and rows).
func LoadPatch(path string) (Layer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Layer{}, fmt.Errorf("boot: read patch %s: %w", path, err)
	}
	var entries []Entry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return Layer{}, fmt.Errorf("boot: parse patch %s: %w", path, err)
	}
	return Layer{Label: path, Entries: entries}, nil
}

// Tree is the composed row list, preserving first-insertion order.
type Tree struct {
	Rows  []Row
	byID  map[string]int
	order []string
}

// NewTree returns an empty tree.
func NewTree() *Tree {
	return &Tree{byID: make(map[string]int)}
}

// Apply applies one layer in order. An insert appends unseen rows; a row with
// an existing id replaces that row's whole definition (last write wins).
func (t *Tree) Apply(layer Layer) error {
	for _, entry := range layer.Entries {
		for _, row := range entry.Insert {
			row.origin = layer.Label
			if err := t.insert(row); err != nil {
				return fmt.Errorf("%s: %w", layer.Label, err)
			}
		}
		if entry.Row.Name == "" && entry.Row.ID == "" {
			continue
		}
		if err := t.patch(entry.Row); err != nil {
			return fmt.Errorf("%s: %w", layer.Label, err)
		}
	}
	return nil
}

func (t *Tree) insert(row Row) error {
	if row.ID == "" {
		return fmt.Errorf("boot: inserted row %q has no id", row.Name)
	}
	if _, exists := t.byID[row.ID]; exists {
		return fmt.Errorf("boot: row %q inserted twice", row.ID)
	}
	t.byID[row.ID] = len(t.Rows)
	t.Rows = append(t.Rows, row)
	if !contains(t.order, row.origin) {
		t.order = append(t.order, row.origin)
	}
	return nil
}

func (t *Tree) patch(row Row) error {
	if row.ID == "" {
		return fmt.Errorf("boot: patch row %q has no id", row.Name)
	}
	index, exists := t.byID[row.ID]
	if !exists {
		return fmt.Errorf("boot: patch targets unknown row %q", row.ID)
	}
	origin := t.Rows[index].origin
	row.origin = origin
	t.Rows[index] = row
	return nil
}

// Dump renders the composed tree grouped by the layer that inserted each row,
// matching the shape of `glh --dump-config`.
func (t *Tree) Dump() (string, error) {
	var builder strings.Builder
	for _, label := range t.order {
		fmt.Fprintf(&builder, "# == %s\n", label)
		var group []Row
		for _, row := range t.Rows {
			if row.origin == label {
				group = append(group, row)
			}
		}
		data, err := yaml.Marshal(group)
		if err != nil {
			return "", fmt.Errorf("boot: marshal tree: %w", err)
		}
		builder.Write(data)
	}
	return builder.String(), nil
}

// ProfileDir returns $GLH_HOME/profiles/<name>.
func ProfileDir(home, name string) string {
	return filepath.Join(home, "profiles", name)
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
