package config

import "testing"

func TestValidateRedisTLSForEnvironment_AllowsLocalDevelopmentWithoutTLS(t *testing.T) {
	t.Parallel()

	err := validateRedisTLSForEnvironment(
		EnvironmentDevelopment,
		true,
		"localhost",
		false,
		false,
	)
	if err != nil {
		t.Fatalf("expected local development redis without TLS to be allowed: %v", err)
	}
}

func TestValidateRedisTLSForEnvironment_RejectsRemoteRedisWithoutTLSInStagingAndProduction(t *testing.T) {
	t.Parallel()

	for _, env := range []string{EnvironmentStaging, EnvironmentProduction} {
		env := env
		t.Run(env, func(t *testing.T) {
			t.Parallel()

			err := validateRedisTLSForEnvironment(
				env,
				true,
				"redis.example.internal",
				false,
				false,
			)
			if err == nil {
				t.Fatalf("expected %s to reject remote redis without TLS", env)
			}
		})
	}
}

func TestValidateRedisTLSForEnvironment_RejectsInsecureSkipVerifyInStagingAndProduction(t *testing.T) {
	t.Parallel()

	for _, env := range []string{EnvironmentStaging, EnvironmentProduction} {
		env := env
		t.Run(env, func(t *testing.T) {
			t.Parallel()

			err := validateRedisTLSForEnvironment(
				env,
				true,
				"redis.example.internal",
				true,
				true,
			)
			if err == nil {
				t.Fatalf("expected %s to reject REDIS_TLS_INSECURE_SKIP_VERIFY=true", env)
			}
		})
	}
}

func TestBuildRedisTLSConfig_ReturnsTLSConfigWhenEnabled(t *testing.T) {
	t.Parallel()

	tlsConfig, err := BuildRedisTLSConfig(true, "redis.example.internal", "", false)
	if err != nil {
		t.Fatalf("expected TLS config to build: %v", err)
	}
	if tlsConfig == nil {
		t.Fatal("expected non-nil TLS config when REDIS_TLS_ENABLED=true")
	}
	if tlsConfig.ServerName != "redis.example.internal" {
		t.Fatalf("expected server name to be set, got %q", tlsConfig.ServerName)
	}
	if tlsConfig.MinVersion == 0 {
		t.Fatal("expected minimum TLS version to be configured")
	}
}
