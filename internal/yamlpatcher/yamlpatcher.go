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

func (y *YAMLPatcher) Run(ctx context.Context, r io.Reader, w io.Writer) (int, error) {
	objectsWritten := 0

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

	slog.Info("running YAML patcher", slog.Int("patchCount", y.patches.PatchCount()))

	for {
		var obj *unstructured.Unstructured
		if err := decoder.Decode(&obj); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return 0, fmt.Errorf("failed to decode raw document: %w", err)
		}

		newObjs, err := y.patches.Apply(ctx, obj)
		if err != nil {
			return 0, fmt.Errorf(
				"error patching input object (gvk: '%s' namespace: '%s' name: '%s'): %w",
				obj.GroupVersionKind(), obj.GetNamespace(), obj.GetName(), err,
			)
		}

		for _, newObj := range newObjs {
			if err := encoder.Encode(newObj); err != nil {
				return 0, fmt.Errorf("failed write output: %w", err)
			}
		}

		objectsWritten += len(newObjs)
	}

	return objectsWritten, nil
}
