package envroot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMonorepoRootFrom(t *testing.T) {
	t.Parallel()
	here, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// apps/goapi when tests run from module root
	root, err := MonorepoRootFrom(here)
	if err != nil {
		t.Fatal(err)
	}
	if root == "" {
		t.Fatal("expected monorepo root from apps/goapi working directory")
	}
	if !fileExists(filepath.Join(root, "pnpm-workspace.yaml")) {
		t.Fatalf("missing pnpm-workspace.yaml under %q", root)
	}
	if !fileExists(filepath.Join(root, ".env.example")) {
		t.Fatalf("missing root .env.example under %q", root)
	}
}

func TestMonorepoRootFrom_UnrelatedPath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	root, err := MonorepoRootFrom(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if root != "" {
		t.Fatalf("expected empty root, got %q", root)
	}
}
