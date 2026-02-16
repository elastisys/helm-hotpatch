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

const (
	DirNameAdd    = "add"
	DirNamePatch  = "patch"
	DirNameRemove = "remove"
)

func LoadPatchFromFile(path string) (*unstructured.Unstructured, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	var obj *unstructured.Unstructured

	decoder := yaml.NewYAMLOrJSONDecoder(f, 4096)

	if err := decoder.Decode(&obj); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return obj, nil
}

func LoadPatchesFromDir(ctx context.Context, rootPath string) (Patches, error) {
	var patches Patches

	if err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk: %w", err)
		}

		if !d.IsDir() {
			return nil
		}

		switch d.Name() {
		case DirNameAdd:
			objs, err := loadObjsFromDir(ctx, path)
			if err != nil {
				return fmt.Errorf("load additions from '%s': %w", path, err)
			}
			patches.Add = append(patches.Add, objs...)
		case DirNamePatch:
			objs, err := loadObjsFromDir(ctx, path)
			if err != nil {
				return fmt.Errorf("load patches from '%s': %w", path, err)
			}
			patches.Patch = append(patches.Patch, objs...)
		case DirNameRemove:
			objs, err := loadObjsFromDir(ctx, path)
			if err != nil {
				return fmt.Errorf("load removals from '%s': %w", path, err)
			}
			patches.Remove = append(patches.Remove, objs...)
		}

		return nil
	}); err != nil {
		return Patches{}, fmt.Errorf("walk patch directory: %w", err)
	}

	return patches, nil
}

func loadObjsFromDir(ctx context.Context, rootPath string) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured

	if err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk: %w", err)
		}

		if d.IsDir() {
			return nil
		}

		slog.DebugContext(ctx, "found patch", slog.String("path", path))

		obj, err := LoadPatchFromFile(path)
		if err != nil {
			return fmt.Errorf("load patch '%s': %w", path, err)
		}

		slog.DebugContext(ctx, "loaded patch", slog.String("path", path), slog.String("kind", obj.GetKind()))

		objs = append(objs, obj)

		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk patch directory: %w", err)
	}

	return objs, nil
}
