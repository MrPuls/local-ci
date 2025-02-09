package config

import (
	"fmt"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	cfg := Config{}
	err := cfg.GetConfig("../../internal/test/local.yaml")
	if err != nil {
		t.Errorf("error parsing yaml: %v", err)
	}

	validationErr := ValidateConfig(cfg)
	t.Log(cfg)
	if validationErr != nil {
		t.Errorf("error validating yaml: %v", validationErr)
	}

}

func TestParseVariable(t *testing.T) {
	cfg := Config{}
	err := cfg.GetConfig("../../internal/test/local.yaml")
	if err != nil {
		t.Error("error parsing yaml")
	}
	var variables []string
	for k, v := range cfg.Blocks["Test"].Variables {
		variables = append(variables, fmt.Sprintf("%s=%s", k, v))
	}
	t.Log(cfg.Blocks)
	t.Log(variables)

	if len(variables) != len(cfg.Blocks["Test"].Variables) {
		t.Error("variables were not parsed")
	}

}

func TestParseGlobalVariables(t *testing.T) {
	cfg := Config{}
	err := cfg.GetConfig("../../internal/test/local.yaml")
	if err != nil {
		t.Error("error parsing yaml")
	}
	t.Log(cfg.GlobalVariables)

	if cfg.GlobalVariables["FOO"] != "Im a global variable too!" {
		t.Error("global variable FOO not parsed")
	}
}

func TestParseCache(t *testing.T) {
	cfg := Config{}
	err := cfg.GetConfig("../../internal/test/local.yaml")
	if err != nil {
		t.Error("error parsing yaml")
	}
	if cfg.Blocks["Test"].Cache.Key != "" && len(cfg.Blocks["Test"].Cache.Paths) != 0 {
		if cfg.Blocks["Test"].Cache.Key != "deps" {
			t.Errorf("cache key invalid, expected 'deps', got '%v'", cfg.Blocks["Test"].Cache.Key)
		}

		if cfg.Blocks["Test"].Cache.Paths[0] != "./venv" {
			t.Errorf("cache path value invalid, expected './venv', got '%v'", cfg.Blocks["Test"].Cache.Paths[0])
		}
	}
}
