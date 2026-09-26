package config

import "testing"

func TestProductionRejectsInsecureSettings(t *testing.T) {
	for _, cfg := range []Config{
		{AppEnv: "production", JWTSecret: "dev-secret-change-me"},
		{AppEnv: "production", JWTSecret: "unique-secret", GigaChatInsecure: true},
	} {
		if err := cfg.Validate(); err == nil {
			t.Errorf("expected validation error for %+v", cfg)
		}
	}
	if err := (&Config{AppEnv: "production", JWTSecret: "unique-secret"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsUnknownPointsNamespace(t *testing.T) {
	if err := (&Config{AppEnv: "development", PointsNamespace: "other"}).Validate(); err == nil {
		t.Fatal("unknown points namespace was accepted")
	}
}
