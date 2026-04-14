package yamlpatcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const helmPostRendererFilenameAnnotation = "postrenderer.helm.sh/postrender-filename"

type YAMLPatcher struct {
	patches PatchMap
}

func NewYAMLPatcher(patches PatchMap) *YAMLPatcher {
	return &YAMLPatcher{patches: patches}
}

func (y *YAMLPatcher) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	decoder := yaml.NewYAMLOrJSONDecoder(r, 4096)
	encoder := streaming.NewEncoder(json.YAMLFramer.NewFrameWriter(w), json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		nil,
		nil,
		json.SerializerOptions{
			Yaml:   true,
			Strict: false,
		},
	))

	slog.InfoContext(
		ctx,
		"running YAML patcher",
		slog.Int("patchCount", len(y.patches)),
	)

out:
	for {
		var obj *unstructured.Unstructured
		if err := decoder.Decode(&obj); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to decode raw document: %w", err)
		}

		target := PatchTarget(obj.GetAnnotations()[helmPostRendererFilenameAnnotation])
		if target == "" {
			return fmt.Errorf(
				"input object (gvk: '%s' namespace: '%s' name: '%s') missing annotation %s",
				obj.GroupVersionKind(), obj.GetNamespace(), obj.GetName(), helmPostRendererFilenameAnnotation,
			)
		}

		objLog := slog.With(
			slog.String("group", obj.GroupVersionKind().Group),
			slog.String("version", obj.GroupVersionKind().Version),
			slog.String("kind", obj.GroupVersionKind().Kind),
			slog.String("namespace", obj.GetNamespace()),
			slog.String("name", obj.GetName()),
			slog.String("target", string(target)),
		)

		objLog.DebugContext(ctx, "processing")

		for _, p := range y.patches[target] {
			switch p.Action {
			case PatchActionAdd:
				if err := encoder.Encode(p.Data); err != nil {
					return fmt.Errorf("failed write output: %w", err)
				}

			case PatchActionPatch:
				if patchApplies(obj, p.Data) {
					merge(obj.Object, p.Data.Object)
				}

			case PatchActionRemove:
				if patchApplies(obj, p.Data) {
					continue out
				}
			}
		}

		if err := encoder.Encode(obj); err != nil {
			return fmt.Errorf("failed write output: %w", err)
		}
	}

	return nil
}

func patchApplies(obj, patchObj *unstructured.Unstructured) bool {
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
