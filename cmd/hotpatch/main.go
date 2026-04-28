package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/elastisys/helm-hotpatch/internal/yamlpatcher"
)

var rootPath string

func init() {
	flag.StringVar(&rootPath, "path", "./patches", "path to patches directory")

	if val, ok := os.LookupEnv("HELM_HOTPATCH_PATH"); ok {
		rootPath = val
	}
}

func run(ctx context.Context) error {
	// If the patch directory is not found, just pipe.
	if _, err := os.Stat(rootPath); errors.Is(err, os.ErrNotExist) {
		slog.DebugContext(ctx, "patches directory not found", slog.String("path", rootPath), slog.String("error", err.Error()))

		if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
			return fmt.Errorf("copying stdin to stdout: %w", err)
		}
		return nil
	}

	patches, err := yamlpatcher.LoadPatchMapFromDir(ctx, rootPath)
	if err != nil {
		return fmt.Errorf("load patches: %w", err)
	}

	yp := yamlpatcher.NewYAMLPatcher(patches)

	objectsWritten, err := yp.Run(ctx, os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("process: %w", err)
	}

	// Helm thinks empty output from a post-renderer is an error. While this
	// is probably usually the case, there are actually legitimate cases. For
	// example:
	// * The Helm chart that is being rendered is itself empty.
	// * Patches remove all objects.
	// To work around this issue we print a single YAML null.
	// See: https://github.com/helm/helm/issues/31894
	if objectsWritten == 0 {
		os.Stdout.Write([]byte("null"))
	}

	return nil
}

func main() {
	flag.Parse()

	slog.SetLogLoggerLevel(slog.LevelDebug)

	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error:  %s\n", err)
		os.Exit(1)
	}
}
