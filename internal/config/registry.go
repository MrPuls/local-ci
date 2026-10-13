package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/distribution/reference"
	"go.yaml.in/yaml/v4"
)

// Registries matches credentials to exact registry hosts, including any port.
type Registries map[string]RegistryConfig

type RegistryConfig struct {
	Username    string `yaml:"username,omitempty"`
	Password    string `yaml:"password,omitempty" json:"-"`
	UsernameEnv string `yaml:"username_env,omitempty"`
	PasswordEnv string `yaml:"password_env,omitempty"`
}

func (r *RegistryConfig) UnmarshalYAML(node *yaml.Node) error {
	type plain RegistryConfig
	var decoded plain
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("registry credentials must be a mapping")
	}
	if err := node.Decode(&decoded); err != nil {
		// YAML's type errors can contain the supplied credential value.
		return fmt.Errorf("registry username, password, username_env and password_env must be strings")
	}
	*r = RegistryConfig(decoded)
	return nil
}

func (r *Registries) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("registries must be a mapping of registry hosts to credentials")
	}
	var raw map[string]RegistryConfig
	if err := node.Decode(&raw); err != nil {
		return err
	}
	decoded := make(Registries, len(raw))
	for host, credentials := range raw {
		normalized, err := normalizeRegistryHost(host)
		if err != nil {
			return err
		}
		if _, exists := decoded[normalized]; exists {
			return fmt.Errorf("duplicate registry host %q", normalized)
		}
		decoded[normalized] = credentials
	}
	*r = decoded
	return nil
}

func normalizeRegistryHost(host string) (string, error) {
	host = strings.ToLower(host)
	if host == "" || strings.ContainsAny(host, "/@ \t\r\n") {
		return "", fmt.Errorf("registry key must be a hostname with an optional port, without a URL scheme or path")
	}
	ref, err := reference.ParseNormalizedNamed(host + "/local-ci-registry-check")
	if err != nil || (reference.Domain(ref) != host && host != "index.docker.io") {
		return "", fmt.Errorf("registry key must be a hostname with an optional port, such as registry.example.com or localhost:5000")
	}
	return reference.Domain(ref), nil
}

var registryEnvName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func (r RegistryConfig) Validate() error {
	if (r.Username == "") == (r.UsernameEnv == "") {
		return fmt.Errorf("specify exactly one of username or username_env")
	}
	if r.Username != "" && strings.TrimSpace(r.Username) == "" {
		return fmt.Errorf("username must not be blank")
	}
	if (r.Password == "") == (r.PasswordEnv == "") {
		return fmt.Errorf("specify exactly one of password or password_env")
	}
	if r.UsernameEnv != "" && !registryEnvName.MatchString(r.UsernameEnv) {
		return fmt.Errorf("username_env must be an environment variable name")
	}
	if r.PasswordEnv != "" && !registryEnvName.MatchString(r.PasswordEnv) {
		return fmt.Errorf("password_env must be an environment variable name")
	}
	return nil
}

// Resolve reads host environment variables only when an image needs pulling.
// Literal values are not expanded, so passwords containing '$' work unchanged.
func (r RegistryConfig) Resolve() (username, password string, err error) {
	if err := r.Validate(); err != nil {
		return "", "", err
	}
	username, password = r.Username, r.Password
	if r.UsernameEnv != "" {
		username = os.Getenv(r.UsernameEnv)
		if strings.TrimSpace(username) == "" {
			return "", "", fmt.Errorf("registry username environment variable %q is unset or empty", r.UsernameEnv)
		}
	}
	if r.PasswordEnv != "" {
		password = os.Getenv(r.PasswordEnv)
		if password == "" {
			return "", "", fmt.Errorf("registry password environment variable %q is unset or empty", r.PasswordEnv)
		}
	}
	return username, password, nil
}

func (v *ConfigValidator) validateRegistries() error {
	return v.cfg.Registries.Validate()
}

func (r Registries) Validate() error {
	for host, credentials := range r {
		normalized, err := normalizeRegistryHost(host)
		if err != nil {
			return err
		}
		if normalized != host {
			return fmt.Errorf("registry host must use its canonical name %q", normalized)
		}
		if err := credentials.Validate(); err != nil {
			return fmt.Errorf("registry %q: %w", host, err)
		}
	}
	return nil
}
