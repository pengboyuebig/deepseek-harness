package boot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PackageManifest is the subset of a package.json the loader reads: the
// `dsh.profile` bundles a profile stacks and the `dsh.bundle` patch a bundle
// points at.
type PackageManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	DSH     struct {
		Bundle struct {
			Patch     PatchList `json:"patch"`
			Platforms []string  `json:"platforms"`
		} `json:"bundle"`
		Profile struct {
			Bundles []string `json:"bundles"`
		} `json:"profile"`
	} `json:"dsh"`
}

// PatchList accepts the string and array forms of `dsh.bundle.patch`.
type PatchList []string

// UnmarshalJSON accepts either a single path or an ordered array of paths.
func (p *PatchList) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*p = PatchList{single}
		return nil
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	*p = PatchList(list)
	return nil
}

// ReadManifest loads and validates a package.json.
func ReadManifest(path string) (PackageManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PackageManifest{}, fmt.Errorf("boot: read manifest %s: %w", path, err)
	}
	var manifest PackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return PackageManifest{}, fmt.Errorf("boot: parse manifest %s: %w", path, err)
	}
	return manifest, nil
}

// ProfileBundles returns the ordered bundle names declared by a profile.
func ProfileBundles(profileDir string) ([]string, error) {
	manifest, err := ReadManifest(filepath.Join(profileDir, "package.json"))
	if err != nil {
		return nil, err
	}
	return manifest.DSH.Profile.Bundles, nil
}

// ResolvePackageDir finds an installed package directory by walking up from the
// install anchor, then falling back to the profile's own node_modules.
func ResolvePackageDir(name, installAnchor, profileDir string) (string, error) {
	relative := filepath.FromSlash(name)
	if profileDir != "" {
		candidate := filepath.Join(profileDir, "node_modules", relative)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
	}
	dir := installAnchor
	for {
		candidate := filepath.Join(dir, "node_modules", relative)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("boot: cannot resolve bundle %q from %s", name, installAnchor)
}

// BundleLayers resolves each bundle in order to the layer its `dsh.bundle.patch`
// declares, labelled by the package name as the runtime dump prints it.
func BundleLayers(bundles []string, installAnchor, profileDir string) ([]Layer, error) {
	layers := make([]Layer, 0, len(bundles))
	for _, name := range bundles {
		dir, err := ResolvePackageDir(name, installAnchor, profileDir)
		if err != nil {
			return nil, err
		}
		manifest, err := ReadManifest(filepath.Join(dir, "package.json"))
		if err != nil {
			return nil, err
		}
		patches := manifest.DSH.Bundle.Patch
		if len(patches) == 0 {
			return nil, fmt.Errorf("boot: %s declares no dsh.bundle.patch", name)
		}
		label := manifest.Name
		if label == "" {
			label = name
		}
		for _, patch := range patches {
			layer, err := LoadPatch(filepath.Join(dir, filepath.FromSlash(patch)))
			if err != nil {
				return nil, err
			}
			layer.Label = label
			layers = append(layers, layer)
		}
	}
	return layers, nil
}

// StackLayers composes the full layer order: profile bundles, the profile patch,
// the home patch, then `--patch` overlays. The install anchor is the directory
// whose node_modules tree holds the bundle packages (normally the CLI package).
func StackLayers(profileDir, home, installAnchor string, overlays []string) ([]Layer, error) {
	bundles, err := ProfileBundles(profileDir)
	if err != nil {
		return nil, err
	}
	layers, err := BundleLayers(bundles, installAnchor, profileDir)
	if err != nil {
		return nil, err
	}

	for _, candidate := range []string{
		filepath.Join(profileDir, "cordis.patch.yml"),
		filepath.Join(home, "cordis.patch.yml"),
	} {
		if _, err := os.Stat(candidate); err != nil {
			continue
		}
		layer, err := LoadPatch(candidate)
		if err != nil {
			return nil, err
		}
		layers = append(layers, layer)
	}
	for _, overlay := range overlays {
		layer, err := LoadPatch(overlay)
		if err != nil {
			return nil, err
		}
		layers = append(layers, layer)
	}
	return layers, nil
}
