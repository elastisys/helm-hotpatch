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

	applied bool
}

type PatchList []*Patch

type Patches struct {
	Target  PatchTarget `json:"target"`
	Patches []*Patch    `json:"patches"`
}

type PatchMap map[PatchTarget]PatchList

func (pm PatchMap) PatchCount() (count int) {
	for _, pl := range pm {
		count += len(pl)
	}
	return
}

func (pm PatchMap) Apply(ctx context.Context, obj *unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	target := PatchTarget(obj.GetAnnotations()[helmPostRendererFilenameAnnotation])
	if target == "" {
		return nil, fmt.Errorf("missing annotation %s", helmPostRendererFilenameAnnotation)
	}

	pl := pm[target]

	if len(pl) == 0 {
		return []*unstructured.Unstructured{obj}, nil
	}

	newObjs, err := pl.Apply(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("apply patch list: %w", err)
	}

	return newObjs, nil
}

func (pl PatchList) Apply(ctx context.Context, obj *unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	newObjs := []*unstructured.Unstructured{}

	var removed bool

	objLog := slog.With(
		slog.Group("obj",
			slog.String("group", obj.GroupVersionKind().Group),
			slog.String("version", obj.GroupVersionKind().Version),
			slog.String("kind", obj.GroupVersionKind().Kind),
			slog.String("namespace", obj.GetNamespace()),
			slog.String("name", obj.GetName()),
		),
	)

	for _, p := range pl {
		patchLog := objLog.With(
			slog.Group("patch",
				slog.String("action", string(p.Action)),
				slog.String("group", p.Data.GroupVersionKind().Group),
				slog.String("version", p.Data.GroupVersionKind().Version),
				slog.String("kind", p.Data.GroupVersionKind().Kind),
				slog.String("namespace", p.Data.GetNamespace()),
				slog.String("name", p.Data.GetName()),
			),
		)

		patchLog.DebugContext(ctx, "processing patch")

		isMatch := objMatch(obj, p.Data)

		switch p.Action {
		case PatchActionAdd:
			if isMatch {
				return nil, fmt.Errorf("trying to add object that already exists")
			} else if !p.applied {
				patchLog.DebugContext(ctx, "applying patch")
				p.applied = true
				newObjs = append(newObjs, p.Data)
			} else {
				patchLog.DebugContext(ctx, "already applied")
			}
		case PatchActionPatch:
			if isMatch {
				patchLog.DebugContext(ctx, "applying patch")
				p.applied = true
				merge(obj.Object, p.Data.Object)
			} else {
				patchLog.DebugContext(ctx, "does not apply")
			}
		case PatchActionRemove:
			if isMatch {
				patchLog.DebugContext(ctx, "applying patch")
				p.applied = true
				removed = true
			} else {
				patchLog.DebugContext(ctx, "does not apply")
			}
		default:
			return nil, fmt.Errorf("unsupported patch action: %s", p.Action)
		}
	}

	if !removed {
		newObjs = append(newObjs, obj)
	}

	return newObjs, nil
}

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

func objMatch(obj, patchObj *unstructured.Unstructured) bool {
	if obj.GroupVersionKind() != patchObj.GroupVersionKind() {
		return false
	}

	if obj.GetNamespace() != patchObj.GetNamespace() {
		return false
	}

	if obj.GetName() != patchObj.GetName() {
		return false
	}

	return true
}

func merge(dst, src map[string]any) {
	for srcKey, srcVal := range src {
		switch srcVal := srcVal.(type) {
		case map[string]any:
			if dstVal, ok := dst[srcKey].(map[string]any); ok {
				merge(dstVal, srcVal)
			} else {
				dst[srcKey] = srcVal
			}
		case []any:
			// TODO: Merge slice elements?
			dst[srcKey] = srcVal
		default:
			dst[srcKey] = srcVal
		}
	}
}
