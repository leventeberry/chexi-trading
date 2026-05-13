package logger

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestLogger_InitializationDoesNotPanic(t *testing.T) {
	t.Parallel()

	Log.Info().Str("test", "logger_smoke").Msg("logger smoke message")
}

func TestLogger_LogLevelParsingViaSubprocess(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		logLevel string
		want     zerolog.Level
	}{
		{name: "debug", logLevel: "debug", want: zerolog.DebugLevel},
		{name: "info_default_unknown", logLevel: "something-else", want: zerolog.InfoLevel},
		{name: "warning_alias", logLevel: "warning", want: zerolog.WarnLevel},
		{name: "error", logLevel: "error", want: zerolog.ErrorLevel},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := runLoggerHelper(tc.logLevel)
			if err != nil {
				t.Fatalf("helper failed: %v output=%s", err, out)
			}
			got := strings.TrimSpace(out)
			want := fmt.Sprintf("%d", int8(tc.want))
			if got != want {
				t.Fatalf("LOG_LEVEL=%q -> level=%q, want %q", tc.logLevel, got, want)
			}
		})
	}
}

func runLoggerHelper(logLevel string) (string, error) {
	cmd := exec.Command(os.Args[0], "-test.run=TestLoggerSubprocessHelper")
	cmd.Env = append(os.Environ(),
		"GO_WANT_LOGGER_HELPER=1",
		"LOG_LEVEL="+logLevel,
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestLoggerSubprocessHelper(t *testing.T) {
	if os.Getenv("GO_WANT_LOGGER_HELPER") != "1" {
		return
	}
	fmt.Printf("%d\n", int8(Log.GetLevel()))
	os.Exit(0)
}
