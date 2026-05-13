package migrations

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var (
	rePGCryptoExt = regexp.MustCompile(`(?is)\bCREATE\s+EXTENSION\s+(?:IF\s+NOT\s+EXISTS\s+)?pgcrypto\b`)
	reGenRandom   = regexp.MustCompile(`(?i)gen_random_uuid\s*\(`)
)

func migrationSQLDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

// TestPGCryptoBeforeGenRandomUUIDInOrderedMigrations ensures every migration that uses
// gen_random_uuid() runs after pgcrypto has been enabled by an earlier migration or earlier in the same file.
func TestPGCryptoBeforeGenRandomUUIDInOrderedMigrations(t *testing.T) {
	dir := migrationSQLDir(t)
	matches, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no *.up.sql files")
	}
	sort.Slice(matches, func(i, j int) bool {
		return migrationVersion(matches[i]) < migrationVersion(matches[j])
	})

	var extensionApplied bool
	for _, path := range matches {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		s := string(body)
		extLoc := rePGCryptoExt.FindStringIndex(s)
		genLoc := reGenRandom.FindStringIndex(s)

		if genLoc != nil && !extensionApplied {
			if extLoc == nil || genLoc[0] < extLoc[0] {
				t.Fatalf("%s: gen_random_uuid appears before CREATE EXTENSION pgcrypto (extension not yet applied from prior migrations)", filepath.Base(path))
			}
		}
		if extLoc != nil {
			extensionApplied = true
		}
	}
	if !extensionApplied {
		t.Fatal("no migration enables pgcrypto (CREATE EXTENSION ... pgcrypto)")
	}
}

func migrationVersion(path string) int {
	base := filepath.Base(path)
	i := strings.Index(base, "_")
	if i <= 0 {
		return 0
	}
	n, err := strconv.Atoi(base[:i])
	if err != nil {
		return 0
	}
	return n
}
