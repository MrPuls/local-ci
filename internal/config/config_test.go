package config

import (
	"fmt"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	cfg := NewConfig("test.yaml")
	err := cfg.LoadConfig()
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
	cfg := NewConfig("test.yaml")
	err := cfg.LoadConfig()
	if err != nil {
		t.Error("error parsing yaml")
	}
	var variables []string
	for k, v := range cfg.Jobs["Test"].Variables {
		variables = append(variables, fmt.Sprintf("%s=%s", k, v))
	}
	t.Log(cfg.Jobs)
	t.Log(variables)

	if len(variables) != len(cfg.Jobs["Test"].Variables) {
		t.Error("variables were not parsed")
	}

}

func TestParseGlobalVariables(t *testing.T) {
	cfg := NewConfig("test.yaml")
	err := cfg.LoadConfig()
	if err != nil {
		t.Error("error parsing yaml")
	}
	t.Log(cfg.GlobalVariables)

	if cfg.GlobalVariables["FOO"] != "Im a global variable too!" {
		t.Error("global variable FOO not parsed")
	}
}

func TestParseCache(t *testing.T) {
	cfg := NewConfig("test.yaml")
	err := cfg.LoadConfig()
	if err != nil {
		t.Error("error parsing yaml")
	}
	// TODO: This should be a different scenarios, either a different yaml or a mock
	// 	with the check like "if cache is present in yaml in should not be nil"
	if cfg.Jobs["Test"].Cache == nil {
		t.Log("cache not present")
	} else {
		if cfg.Jobs["Test"].Cache.Key != "deps" {
			t.Errorf("cache key invalid, expected 'deps', got '%v'", cfg.Jobs["Test"].Cache.Key)
		}

		if cfg.Jobs["Test"].Cache.Paths[0] != "./venv" {
			t.Errorf("cache path value invalid, expected './venv', got '%v'", cfg.Jobs["Test"].Cache.Paths[0])
		}
	}
}

func TestParseNetwork(t *testing.T) {
	cfg := NewConfig("test.yaml")
	err := cfg.LoadConfig()
	if err != nil {
		t.Error("error parsing yaml")
	}
	if cfg.Jobs["Test"].Network == nil {
		t.Log("network not present")
	} else {
		if cfg.Jobs["Test"].Network.HostAccess != true {
			t.Errorf("network host access should be true. got: %v", cfg.Jobs["Test"].Network)
		}
	}
}
