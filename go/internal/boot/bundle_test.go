package boot_test

import (
	"os"
	"path/filepath"
	"testing"

	"git.glkj.example.com/glkj/harness/go/internal/boot"
)

func writePackage(t *testing.T, dir, name, manifest, patch string) {
	t.Helper()
	pkgDir := filepath.Join(dir, "node_modules", filepath.FromSlash(name))
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", pkgDir, err)
	}
	writeFile(t, pkgDir, "package.json", manifest)
	if patch != "" {
		writeFile(t, pkgDir, "cordis.patch.yml", patch)
	}
}

func TestBundleLayersResolveFromTheInstallAnchor(t *testing.T) {
	anchor := t.TempDir()
	writePackage(t, anchor, "@deepseek-ai/dsh-base",
		`{"name":"@deepseek-ai/dsh-base","dsh":{"bundle":{"patch":"./cordis.patch.yml"}}}`,
		"- insert:\n    - id: session\n      name: '@deepseek-ai/dsh-session'\n")
	writePackage(t, anchor, "@deepseek-ai/dsh-web-app",
		`{"name":"@deepseek-ai/dsh-web-app","dsh":{"bundle":{"patch":["./cordis.patch.yml","./presets/standard.patch.yml"]}}}`,
		"- id: session\n  disabled: true\n")
	presetDir := filepath.Join(anchor, "node_modules", "@deepseek-ai", "dsh-web-app", "presets")
	if err := os.MkdirAll(presetDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, presetDir, "standard.patch.yml", "- insert:\n    - id: preset-row\n      name: '@deepseek-ai/dsh-preset'\n")

	profileDir := t.TempDir()
	writeFile(t, profileDir, "package.json",
		`{"name":"p","dsh":{"profile":{"bundles":["@deepseek-ai/dsh-base","@deepseek-ai/dsh-web-app"]}}}`)

	layers, err := boot.BundleLayers([]string{"@deepseek-ai/dsh-base", "@deepseek-ai/dsh-web-app"}, anchor, profileDir)
	if err != nil {
		t.Fatalf("layers: %v", err)
	}
	if len(layers) != 3 {
		t.Fatalf("layers = %d, want 3 (one bundle declares two patches)", len(layers))
	}
	if layers[0].Label != "@deepseek-ai/dsh-base" || layers[1].Label != "@deepseek-ai/dsh-web-app" || layers[2].Label != "@deepseek-ai/dsh-web-app" {
		t.Fatalf("labels = %q, %q, %q", layers[0].Label, layers[1].Label, layers[2].Label)
	}
}

func TestStackLayersOrderIsBundlesProfileHomeOverlays(t *testing.T) {
	anchor := t.TempDir()
	writePackage(t, anchor, "@deepseek-ai/dsh-base",
		`{"name":"@deepseek-ai/dsh-base","dsh":{"bundle":{"patch":"./cordis.patch.yml"}}}`,
		"- insert:\n    - id: session\n      name: '@deepseek-ai/dsh-session'\n")

	home := t.TempDir()
	profileDir := boot.ProfileDir(home, "web")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, profileDir, "package.json", `{"name":"p","dsh":{"profile":{"bundles":["@deepseek-ai/dsh-base"]}}}`)
	writeFile(t, profileDir, "cordis.patch.yml", "- id: session\n  disabled: true\n")
	writeFile(t, home, "cordis.patch.yml", "- id: session\n  disabled: false\n")
	overlay := writeFile(t, home, "overlay.yml", "- insert:\n    - id: overlay\n      name: '@deepseek-ai/dsh-overlay'\n")

	layers, err := boot.StackLayers(profileDir, home, anchor, []string{overlay})
	if err != nil {
		t.Fatalf("stack: %v", err)
	}
	labels := []string{layers[0].Label, layers[1].Label, layers[2].Label, layers[3].Label}
	want := []string{
		"@deepseek-ai/dsh-base",
		filepath.Join(profileDir, "cordis.patch.yml"),
		filepath.Join(home, "cordis.patch.yml"),
		overlay,
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("layer %d = %q, want %q", i, labels[i], want[i])
		}
	}
}

func TestMissingBundleFails(t *testing.T) {
	anchor := t.TempDir()
	profileDir := t.TempDir()
	writeFile(t, profileDir, "package.json", `{"name":"p","dsh":{"profile":{"bundles":["@deepseek-ai/dsh-missing"]}}}`)
	if _, err := boot.StackLayers(profileDir, t.TempDir(), anchor, nil); err == nil {
		t.Fatal("expected a missing bundle to fail")
	}
}
