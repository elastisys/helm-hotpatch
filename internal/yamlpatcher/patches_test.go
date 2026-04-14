package yamlpatcher_test

import (
	"os"
	"testing"

	"github.com/elastisys/helm-hotpatch/internal/yamlpatcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestPatchesYAMLRoundTrip(t *testing.T) {
	testfile := "./testdata/cases/add-patch-remove/patches/patches.yaml"

	expected, err := os.ReadFile(testfile)
	require.NoError(t, err)

	p, err := yamlpatcher.LoadPatchesFromFile(testfile)
	require.NoError(t, err)

	actual, err := yaml.Marshal(p)
	require.NoError(t, err)

	assert.YAMLEq(t, string(expected), string(actual))

}

func TestLoadPatchMapFromDir(t *testing.T) {
	patches, err := yamlpatcher.LoadPatchMapFromDir(t.Context(), "./testdata/cases/add-patch-remove/patches")
	require.NoError(t, err)

	assert.Len(t, patches["foo"], 3)
}
