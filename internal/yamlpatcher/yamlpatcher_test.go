package yamlpatcher_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/elastisys/helm-hotpatch/internal/yamlpatcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYAMLPatcher(t *testing.T) {
	casesPath := "./testdata/cases"
	caseDirs, err := os.ReadDir(casesPath)
	require.NoError(t, err)

	for _, caseDir := range caseDirs {
		casePath := filepath.Join(casesPath, caseDir.Name())
		inputPath := filepath.Join(casePath, "input.yaml")
		expectedPath := filepath.Join(casePath, "expected.yaml")
		patchesPath := filepath.Join(casePath, "patches")

		t.Run(caseDir.Name(), func(t *testing.T) {
			yp := yamlpatcher.NewYAMLPatcher(loadPatchesFromDir(t, patchesPath))

			out := &bytes.Buffer{}
			_, err := yp.Run(t.Context(), openFile(t, inputPath), out)
			require.NoError(t, err)

			if _, ok := os.LookupEnv("HELM_HOTPATCH_TEST_WRITEBACK"); ok {
				f := openFile(t, expectedPath)
				_, err := f.Write(out.Bytes())
				require.NoError(t, err)
			}

			assert.Equal(t, readFile(t, expectedPath), out.String())
		})
	}
}

func openFile(t *testing.T, path string) *os.File {
	t.Helper()

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Logf("error closing test file: %s", err)
		}
	})

	return f
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func loadPatchesFromDir(t *testing.T, path string) yamlpatcher.PatchMap {
	t.Helper()

	patches, err := yamlpatcher.LoadPatchMapFromDir(t.Context(), path)
	require.NoError(t, err)
	return patches
}
