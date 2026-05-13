package initializers

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
	"goapi/config"
	"gorm.io/gorm"
)

func TestGetCacheClientFromRedis_NilReturnsNoOpCache(t *testing.T) {
	t.Parallel()

	c := GetCacheClientFromRedis(nil)
	if c == nil {
		t.Fatal("GetCacheClientFromRedis(nil) returned nil cache")
	}
	if c.SupportsDistributedRateLimit() {
		t.Fatal("nil redis client should produce non-distributed no-op cache")
	}
}

func TestGetCacheClientFromRedis_NonNilReturnsRedisBackedCache(t *testing.T) {
	t.Parallel()

	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = client.Close() })

	c := GetCacheClientFromRedis(client)
	if c == nil {
		t.Fatal("GetCacheClientFromRedis(non-nil) returned nil cache")
	}
	if !c.SupportsDistributedRateLimit() {
		t.Fatal("non-nil redis client should produce distributed cache")
	}
}

func TestCloseRedis_NilIsSafe(t *testing.T) {
	t.Parallel()
	CloseRedis(nil)
}

func TestRunVersionedMigrations_InvalidDirectoryFailsClearly(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Database.User = "u"
	cfg.Database.Pass = "p"
	cfg.Database.Host = "127.0.0.1"
	cfg.Database.Port = "5432"
	cfg.Database.Name = "db"
	cfg.Database.SSLMode = "disable"

	err := RunVersionedMigrations(cfg, "this/path/does/not/exist/for-migrations")
	if err == nil {
		t.Fatal("expected migration error for missing directory")
	}
	msg := err.Error()
	if !strings.Contains(msg, "migrate init") {
		t.Fatalf("error = %q, want migrate init context", msg)
	}
}

func TestMigrateDB_ProductionRefusesAutoMigrateUnlessVersionedEnabled(t *testing.T) {
	output, err := runFatalSubprocess(t, "prod_refuse_automigrate", nil)
	if err == nil {
		t.Fatal("expected subprocess to fail with fatal exit")
	}
	if !strings.Contains(output, "Production requires USE_VERSIONED_MIGRATIONS=true") {
		t.Fatalf("expected production safety message, got: %s", output)
	}
}

func TestConfigLoad_MissingRequiredEnvFailsClearly(t *testing.T) {
	extraEnv := []string{
		"APP_ENV=development",
		"DB_USER=",
		"DB_PASS=",
		"DB_HOST=",
		"DB_NAME=",
		"JWT_SECRET=01234567890123456789012345678901",
	}
	output, err := runFatalSubprocess(t, "missing_required_env", extraEnv)
	if err == nil {
		t.Fatal("expected subprocess to fail with fatal exit")
	}
	if !strings.Contains(output, "Database configuration is incomplete") {
		t.Fatalf("expected clear DB config failure, got: %s", output)
	}
}

func runFatalSubprocess(t *testing.T, testCase string, extraEnv []string) (string, error) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestInitializersFatalSubprocessHelper")
	env := append(os.Environ(),
		"GO_WANT_INITIALIZERS_FATAL_HELPER=1",
		"INITIALIZERS_FATAL_CASE="+testCase,
	)
	env = append(env, extraEnv...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestInitializersFatalSubprocessHelper(t *testing.T) {
	if os.Getenv("GO_WANT_INITIALIZERS_FATAL_HELPER") != "1" {
		return
	}

	switch os.Getenv("INITIALIZERS_FATAL_CASE") {
	case "prod_refuse_automigrate":
		_ = os.Setenv("USE_VERSIONED_MIGRATIONS", "false")
		cfg := &config.Config{}
		cfg.Environment = config.EnvironmentProduction
		migrateDB(&gorm.DB{}, cfg)
		os.Exit(0)
	case "missing_required_env":
		_ = config.Load()
		os.Exit(0)
	default:
		os.Exit(2)
	}
}
