// Package boot composes the plugin tree from ordered profile layers, the Go
// counterpart of packages/boot/app-boot.
package boot

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Row is one plugin row of the composed tree. The decoded YAML node is kept so
// `dump-config` reproduces the source file's tags, key order, and scalar style
// (`!!js` expressions included) instead of normalising them.
type Row struct {
	ID       string
	Name     string
	Inject   []string
	Config   *yaml.Node
	Disabled *Flag

	node    *yaml.Node
	origin  string
	patched []string
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
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!js", Style: yaml.SingleQuotedStyle, Value: f.Expr}, nil
}

// UnmarshalYAML keeps the raw mapping so a dump can reproduce it byte for byte.
func (r *Row) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("boot: row must be a mapping, got %s", node.Tag)
	}
	r.node = cloneNode(node)
	normalizeNode(r.node, false)
	return r.readFields(r.node)
}

// MarshalYAML emits the stored mapping.
func (r Row) MarshalYAML() (any, error) {
	if r.node != nil {
		return r.node, nil
	}
	return r.buildNode(), nil
}

func (r *Row) readFields(node *yaml.Node) error {
	r.ID, r.Name = "", ""
	r.Inject, r.Config, r.Disabled = nil, nil, nil
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		switch key.Value {
		case "id":
			r.ID = value.Value
		case "name":
			r.Name = value.Value
		case "inject":
			if err := value.Decode(&r.Inject); err != nil {
				return fmt.Errorf("boot: row inject: %w", err)
			}
		case "config":
			r.Config = value
		case "disabled":
			var flag Flag
			if err := value.Decode(&flag); err != nil {
				return fmt.Errorf("boot: row disabled: %w", err)
			}
			r.Disabled = &flag
		}
	}
	return nil
}

func (r Row) buildNode() *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	add := func(key string, value *yaml.Node) {
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
	}
	if r.ID != "" {
		add("id", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: r.ID})
	}
	if r.Name != "" {
		add("name", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: r.Name})
	}
	if r.Inject != nil {
		sequence := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, item := range r.Inject {
			sequence.Content = append(sequence.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: item})
		}
		add("inject", sequence)
	}
	if r.Config != nil {
		add("config", r.Config)
	}
	if r.Disabled != nil {
		if r.Disabled.Value != nil {
			value := "false"
			if *r.Disabled.Value {
				value = "true"
			}
			add("disabled", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: value})
		} else {
			add("disabled", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!js", Style: yaml.SingleQuotedStyle, Value: r.Disabled.Expr})
		}
	}
	return node
}

// Entry is one element of a patch file: either an insert block or a row that
// patches an existing row by id.
type Entry struct {
	Insert []Row
	Row    Row
}

// UnmarshalYAML dispatches on the `insert` key so a row's own fields are not
// confused with the entry wrapper.
func (e *Entry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("boot: patch entry must be a mapping, got %s", node.Tag)
	}
	if index := indexOfKey(node, "insert"); index >= 0 {
		return node.Content[index].Decode(&e.Insert)
	}
	return e.Row.UnmarshalYAML(node)
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
	Rows     []Row
	byID     map[string]int
	warnings []string
}

// NewTree returns an empty tree.
func NewTree() *Tree {
	return &Tree{byID: make(map[string]int)}
}

// Apply applies one layer in order. An insert appends unseen rows; a patch
// merges the keys it names into the existing row (config replaces wholesale)
// and records its layer only when the row actually changes.
func (t *Tree) Apply(layer Layer) error {
	for _, entry := range layer.Entries {
		for _, row := range entry.Insert {
			if err := t.insert(row, layer.Label); err != nil {
				return fmt.Errorf("%s: %w", layer.Label, err)
			}
		}
		if entry.Row.node == nil && entry.Row.Name == "" && entry.Row.ID == "" {
			continue
		}
		t.patch(entry.Row, layer.Label)
	}
	return nil
}

func (t *Tree) insert(row Row, label string) error {
	if row.ID == "" {
		return fmt.Errorf("boot: inserted row %q has no id", row.Name)
	}
	if _, exists := t.byID[row.ID]; exists {
		return fmt.Errorf("boot: row %q inserted twice", row.ID)
	}
	row.node = cloneNode(row.node)
	if row.node == nil {
		row.node = row.buildNode()
	}
	row.origin = label
	t.byID[row.ID] = len(t.Rows)
	t.Rows = append(t.Rows, row)
	return nil
}

func (t *Tree) patch(patch Row, label string) {
	if patch.ID == "" {
		t.warnings = append(t.warnings, fmt.Sprintf("%s: patch: entry with no id ignored", label))
		return
	}
	index, exists := t.byID[patch.ID]
	if !exists {
		t.warnings = append(t.warnings, fmt.Sprintf("%s: patch: entry %q not found", label, patch.ID))
		return
	}
	previous := t.Rows[index]
	merged, changed := mergeRow(previous, patch)
	if !changed {
		return
	}
	merged.patched = append([]string(nil), previous.patched...)
	if len(merged.patched) == 0 || merged.patched[len(merged.patched)-1] != label {
		merged.patched = append(merged.patched, label)
	}
	t.Rows[index] = merged
}

// patched is the layer chain recorded on a row; kept off Row's exported surface.
func (r Row) chain() []string { return r.patched }

// Warnings lists patches that targeted a missing row; the runtime prints them
// and keeps going, matching the TypeScript loader.
func (t *Tree) Warnings() []string {
	return append([]string(nil), t.warnings...)
}

// Dump renders the composed tree in insertion order, printing a provenance
// header whenever the layer chain of a row changes, matching the shape of
// `glh --dump-config`.
func (t *Tree) Dump() (string, error) {
	var builder strings.Builder
	var previous []string
	first := true
	for _, row := range t.Rows {
		chain := append([]string{row.origin}, row.chain()...)
		if first || !equalStrings(previous, chain) {
			fmt.Fprintf(&builder, "# == %s\n", strings.Join(chain, ", patched by "))
			previous = chain
			first = false
		}
		data, err := marshalRow(row)
		if err != nil {
			return "", fmt.Errorf("boot: marshal row %q: %w", row.ID, err)
		}
		builder.Write(data)
	}
	return builder.String(), nil
}

// marshalRow emits a one-element list with two-space indentation, matching the
// TypeScript dump.
func marshalRow(row Row) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode([]Row{row}); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// ProfileDir returns $GLH_HOME/profiles/<name>.
func ProfileDir(home, name string) string {
	return filepath.Join(home, "profiles", name)
}

func mergeRow(base, patch Row) (Row, bool) {
	if base.node == nil {
		base.node = base.buildNode()
	}
	mergedNode, changed := mergeNodes(base.node, patch.node)
	merged := base
	merged.node = mergedNode
	if err := merged.readFields(mergedNode); err != nil {
		return base, false
	}
	return merged, changed
}

func mergeNodes(base, patch *yaml.Node) (*yaml.Node, bool) {
	out := cloneNode(base)
	if patch == nil {
		return out, false
	}
	changed := false
	for i := 0; i+1 < len(patch.Content); i += 2 {
		key, value := patch.Content[i], patch.Content[i+1]
		index := indexOfKey(out, key.Value)
		if index < 0 {
			out.Content = append(out.Content, cloneNode(key), cloneNode(value))
			changed = true
			continue
		}
		if !nodesEqual(out.Content[index], value) {
			out.Content[index] = cloneNode(value)
			changed = true
		}
	}
	return out, changed
}

func indexOfKey(node *yaml.Node, key string) int {
	if node == nil || node.Kind != yaml.MappingNode {
		return -1
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return i + 1
		}
	}
	return -1
}

func nodesEqual(left, right *yaml.Node) bool {
	leftValue, err := yaml.Marshal(left)
	if err != nil {
		return false
	}
	rightValue, err := yaml.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftValue) == string(rightValue)
}

// normalizeNode drops source comments and normalises `!!js` quoting so the dump
// matches the runtime's re-emitted form: comments are not part of the tree.
func normalizeNode(node *yaml.Node, key bool) {
	if node == nil {
		return
	}
	node.HeadComment, node.LineComment, node.FootComment = "", "", ""
	quoted := node.Style&yaml.DoubleQuotedStyle != 0
	if node.Tag == "!!js" && quoted {
		node.Style = (node.Style &^ yaml.DoubleQuotedStyle) | yaml.SingleQuotedStyle
	}
	long := len(node.Value) > 78 && !strings.Contains(node.Value, "\n")
	foldable := node.Tag == "!!js" || node.Style == 0
	if node.Kind == yaml.ScalarNode && !key && long && foldable &&
		node.Style&yaml.LiteralStyle == 0 && node.Style&yaml.FoldedStyle == 0 {
		node.Style = (node.Style &^ (yaml.SingleQuotedStyle | yaml.DoubleQuotedStyle)) | yaml.FoldedStyle
	}
	for i, child := range node.Content {
		normalizeNode(child, node.Kind == yaml.MappingNode && i%2 == 0)
	}
}

func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	copied := *node
	copied.Content = make([]*yaml.Node, len(node.Content))
	for i, child := range node.Content {
		copied.Content[i] = cloneNode(child)
	}
	if node.Alias != nil {
		copied.Alias = cloneNode(node.Alias)
	}
	return &copied
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
