//go:build integration

package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testdataPath = "./testdata"
)

var (
	chartPath   = filepath.Join(testdataPath, "chart")
	patchesPath = filepath.Join(testdataPath, "patches")
)

func TestIntegration(t *testing.T) {
	helmDataDir, err := os.MkdirTemp(os.TempDir(), "helm-hotpatch-data-dir-******")
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := os.RemoveAll(helmDataDir); err != nil {
			t.Logf("error removing temporary Helm data directory: %s", err)
		}

		runMake(t, context.Background(), "clean")
	})

	helmPluginsDir := filepath.Join(helmDataDir, "plugins")
	require.NoError(t, os.Mkdir(helmPluginsDir, 0o755))
	// The local installer / dev mode (used when installing from directory) doesn't care about HELM_PLUGINS:
	// https://github.com/helm/helm/blob/4a91f3ad5cc0c1521f6d4dcb5681e2da4baaa157/internal/plugin/installer/local_installer.go#L179
	// The person that fixes it upstream gets a cookie! :)
	os.Setenv("HELM_DATA_HOME", helmDataDir)
	os.Setenv("HELM_PLUGINS", helmPluginsDir)

	runMake(t, t.Context(), "install")

	expected, err := os.ReadFile("./testdata/expected.yaml")
	require.NoError(t, err)

	t.Run("flag", func(t *testing.T) {
		cmd := exec.CommandContext(
			t.Context(),
			"helm",
			"template",
			"--post-renderer",
			"hotpatch",
			"--post-renderer-args",
			"-path",
			"--post-renderer-args",
			patchesPath,
			chartPath,
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		assert.Equal(t, string(expected), string(out))
	})

	t.Run("env", func(t *testing.T) {
		cmd := exec.CommandContext(
			t.Context(),
			"helm",
			"template",
			"--post-renderer",
			"hotpatch",
			chartPath,
		)
		cmd.Env = append(os.Environ(), "HELM_HOTPATCH_PATH="+patchesPath)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		assert.Equal(t, string(expected), string(out))
	})

	t.Run("missing-patches-dir", func(t *testing.T) {
		withoutPostRendering := exec.CommandContext(
			t.Context(),
			"helm",
			"template",
			chartPath,
		)

		withPostRendering := exec.CommandContext(
			t.Context(),
			"helm",
			"template",
			"--post-renderer",
			"hotpatch",
			chartPath,
		)
		withPostRendering.Env = append(os.Environ(), "HELM_HOTPATCH_PATH=./testdata/doesnotexist")

		outWithoutPostRendering, err := withoutPostRendering.CombinedOutput()
		require.NoError(t, err, string(outWithoutPostRendering))

		outWithPostRendering, err := withPostRendering.CombinedOutput()
		require.NoError(t, err, string(outWithPostRendering))

		assert.Equal(t, string(outWithoutPostRendering), string(outWithPostRendering))
	})
}

func runMake(t *testing.T, ctx context.Context, target string) {
	t.Helper()

	cmd := exec.CommandContext(ctx, "make", target)
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}
