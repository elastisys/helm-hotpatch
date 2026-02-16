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

type YAMLPatcher struct {
	patches Patches
}

func NewYAMLPatcher(patches Patches) *YAMLPatcher {
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
		slog.Int("addCount", len(y.patches.Add)),
		slog.Int("patchCount", len(y.patches.Patch)),
		slog.Int("removeCount", len(y.patches.Remove)),
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

		objLog := slog.With(
			slog.String("group", obj.GroupVersionKind().Group),
			slog.String("version", obj.GroupVersionKind().Version),
			slog.String("kind", obj.GroupVersionKind().Kind),
			slog.String("namespace", obj.GetNamespace()),
			slog.String("name", obj.GetName()),
		)

		objLog.DebugContext(ctx, "processing")

		for _, rObj := range y.patches.Remove {
			if patchApplies(obj, rObj) {
				objLog.DebugContext(ctx, "removing")
				continue out
			}
		}

		for _, pObj := range y.patches.Patch {
			if !patchApplies(obj, pObj) {
				continue
			}

			objLog.DebugContext(ctx, "patching")

			merge(obj.Object, pObj.Object)
		}

		if err := encoder.Encode(obj); err != nil {
			return fmt.Errorf("failed write output: %w", err)
		}
	}

	for _, obj := range y.patches.Add {
		slog.DebugContext(
			ctx,
			"adding",
			slog.String("group", obj.GroupVersionKind().Group),
			slog.String("version", obj.GroupVersionKind().Version),
			slog.String("kind", obj.GroupVersionKind().Kind),
			slog.String("namespace", obj.GetNamespace()),
			slog.String("name", obj.GetName()),
		)

		if err := encoder.Encode(obj); err != nil {
			return fmt.Errorf("failed write patch to output: %w", err)
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
