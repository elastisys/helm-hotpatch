package yamlpatcher

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type PatchTarget string

type PatchAction string

const (
	PatchActionAdd    PatchAction = "add"
	PatchActionPatch  PatchAction = "patch"
	PatchActionRemove PatchAction = "remove"
)

type Patch struct {
	Action PatchAction                `json:"action"`
	Data   *unstructured.Unstructured `json:"data"`
}

type Patches struct {
	Target  PatchTarget `json:"target"`
	Patches []Patch     `json:"patches"`
}

type PatchMap map[PatchTarget][]Patch

func LoadPatchesFromFile(path string) (Patches, error) {
	f, err := os.Open(path)
	if err != nil {
		return Patches{}, fmt.Errorf("open: %w", err)
	}

	var p Patches

	decoder := yaml.NewYAMLOrJSONDecoder(f, 4096)

	if err := decoder.Decode(&p); err != nil {
		return Patches{}, fmt.Errorf("YAML decode: %w", err)
	}

	return p, nil
}

func LoadPatchMapFromDir(ctx context.Context, rootPath string) (PatchMap, error) {
	patches := PatchMap{}

	slog.DebugContext(ctx, "trying to load patches from directory", slog.String("path", rootPath))

	if err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk: %w", err)
		}

		if d.IsDir() {
			return nil
		}

		slog.DebugContext(ctx, "found patch file", slog.String("path", path))

		ps, err := LoadPatchesFromFile(path)
		if err != nil {
			return fmt.Errorf("load patch '%s': %w", path, err)
		}

		for _, p := range ps.Patches {
			slog.DebugContext(ctx, "loaded patch", slog.String("path", path), slog.String("action", string(p.Action)), slog.String("kind", p.Data.GetKind()))

			patches[ps.Target] = append(patches[ps.Target], p)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk patch directory: %w", err)
	}

	return patches, nil
}
