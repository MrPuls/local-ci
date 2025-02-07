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
