package config

import (
	"fmt"
	"slices"
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
		if cfg.Jobs["Test"].Cache.Key != "cache" {
			t.Errorf("cache key invalid, expected 'deps', got '%v'", cfg.Jobs["Test"].Cache.Key)
		}

		for _, v := range cfg.Jobs["Test"].Cache.Paths {
			if !slices.Contains([]string{".npm", "node_modules"}, v) {
				t.Errorf("cache path value invalid, expected '[.npm, node_modules]', got '%v'", v)
			}
		}
	}
}

func TestParseNetworkHostAccess(t *testing.T) {
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

func TestParseNetworkHostMode(t *testing.T) {
	cfg := NewConfig("test.yaml")
	err := cfg.LoadConfig()
	if err != nil {
		t.Error("error parsing yaml")
	}
	if cfg.Jobs["Test3"].Network == nil {
		t.Log("network not present")
	} else {
		if cfg.Jobs["Test3"].Network.HostMode != true {
			t.Errorf("network host mode should be true. got: %v", cfg.Jobs["Test3"].Network)
		}
	}
}

func TestParseScript(t *testing.T) {
	cfg := NewConfig("test.yaml")
	expectedCommands := []string{
		"echo \"Hello World\"",
		"echo $FOO",
		"touch foo.txt",
		"sleep 5",
		"echo \"Hello from txt file\" >> foo.txt",
		"echo $BAZ >> foo.txt",
		"cat foo.txt",
	}

	err := cfg.LoadConfig()
	if err != nil {
		t.Error("error parsing yaml")
	}
	if cfg.Jobs["Test"].Script == nil {
		t.Log("script not present")
	} else {
		if len(cfg.Jobs["Test"].Script) != len(expectedCommands) {
			t.Errorf("expected %v commands, got %v", len(expectedCommands), len(cfg.Jobs["Test"].Script))
		}
	}
}

func TestVariableOverrides(t *testing.T) {
	cfg := NewConfig("test.yaml")
	err := cfg.LoadConfig()
	if err != nil {
		t.Error("error parsing yaml")
	}

	if cfg.Jobs["Test3"].Variables["FOO"] == cfg.GlobalVariables["FOO"] {
		t.Errorf("local variable FOO should override global variable, expected %v, got %v", cfg.Jobs["Test3"].Variables["FOO"], cfg.GlobalVariables["FOO"])
	}

	if cfg.Jobs["Test3"].Variables["BAZ"] == cfg.GlobalVariables["BAZ"] {
		t.Errorf("local variable BAZ should not override global variable, expected %v, got %v", cfg.Jobs["Test3"].Variables["BAZ"], cfg.GlobalVariables["BAZ"])
	}
}

func TestCommentedJobsParsing(t *testing.T) {
	cfg := NewConfig("test.yaml")
	err := cfg.LoadConfig()
	if err != nil {
		t.Error("error parsing yaml")
	}

	if _, ok := cfg.Jobs["Test2"]; ok {
		t.Error("commented job should not be parsed")
	}
}
