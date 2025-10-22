package env

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var envPattern = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)

// Config controls environment variable expansion behaviour.
type Config struct {
	Files []string
}

// Resolver expands ${VAR} references using process env or provided env files.
type Resolver struct {
	cfg Config
	env map[string]string
}

// NewResolver builds a Resolver and eagerly loads .env style files.
func NewResolver(cfg Config) (*Resolver, error) {
	envMap := make(map[string]string)
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		envMap[parts[0]] = parts[1]
	}

	for _, file := range cfg.Files {
		if err := loadEnvFile(envMap, file); err != nil {
			return nil, fmt.Errorf("load env file %s: %w", file, err)
		}
	}

	return &Resolver{cfg: cfg, env: envMap}, nil
}

// Expand replaces ${VAR} occurrences with corresponding values.
func (r *Resolver) Expand(data []byte) ([]byte, error) {
	return envPattern.ReplaceAllFunc(data, func(match []byte) []byte {
		groups := envPattern.FindSubmatch(match)
		if len(groups) != 2 {
			return match
		}
		key := string(groups[1])
		if val, ok := r.env[key]; ok {
			return []byte(val)
		}
		return match
	}), nil
}

func loadEnvFile(target map[string]string, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid env entry %q on line %d", line, lineNo)
		}
		target[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

// ExpandString helper for tests.
func (r *Resolver) ExpandString(input string) (string, error) {
	data, err := r.Expand([]byte(input))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ExpandMap applies expansion to all string leaf values of the map.
func (r *Resolver) ExpandMap(values map[string]any) error {
	for key, val := range values {
		switch typed := val.(type) {
		case string:
			expanded, err := r.Expand([]byte(typed))
			if err != nil {
				return fmt.Errorf("expand key %s: %w", key, err)
			}
			values[key] = string(expanded)
		case map[string]any:
			if err := r.ExpandMap(typed); err != nil {
				return err
			}
		case []any:
			for i, item := range typed {
				if str, ok := item.(string); ok {
					expanded, err := r.Expand([]byte(str))
					if err != nil {
						return fmt.Errorf("expand list item %s[%d]: %w", key, i, err)
					}
					typed[i] = string(expanded)
				} else if nested, ok := item.(map[string]any); ok {
					if err := r.ExpandMap(nested); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// ExpandBytes convenience function to expand without creating resolver instance.
func ExpandBytes(data []byte, files ...string) ([]byte, error) {
	resolver, err := NewResolver(Config{Files: files})
	if err != nil {
		return nil, err
	}
	return resolver.Expand(data)
}
