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
	for _, v := range cfg.Jobs {
		fmt.Println(v.Network)
	}
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
	for _, j := range cfg.Jobs {
		var variables []string
		if j.Variables != nil {
			for k, v := range j.Variables {
				variables = append(variables, fmt.Sprintf("%s=%s", k, v))
			}
			t.Log(cfg.Jobs)
			t.Log(variables)

			if len(variables) != len(j.Variables) {
				t.Error("variables were not parsed")
			}
		}
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
