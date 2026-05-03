package core

import (
	"os"
	"strconv"
	"strings"
)

type Config map[string]any

type ConfigField struct {
	Path        string
	Flag        string
	Required    bool
	Secret      bool
	Description string
	Env         []string
}

type ConfigSpec struct {
	ProviderID ProviderID
	Fields     []ConfigField
}

func ConfigFromEnv() Config {
	env := make(map[string]any)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		env[key] = value
	}
	return Config{"env": env}
}

func (c Config) Lookup(path ...string) (any, bool) {
	if len(path) == 0 {
		return map[string]any(c), true
	}

	var current any = map[string]any(c)
	for _, key := range path {
		next, ok := lookupConfigValue(current, key)
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func (c Config) String(path ...string) string {
	value, ok := c.Lookup(path...)
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func (c Config) Bool(path ...string) (bool, bool) {
	value, ok := c.Lookup(path...)
	if !ok {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return false, false
		}
		return parsed, true
	default:
		return false, false
	}
}

func (c Config) Env(key string) string {
	if value := c.String("env", key); value != "" {
		return value
	}
	return c.String(key)
}

func lookupConfigValue(value any, key string) (any, bool) {
	switch typed := value.(type) {
	case Config:
		next, ok := typed[key]
		return next, ok
	case map[string]any:
		next, ok := typed[key]
		return next, ok
	case map[string]string:
		next, ok := typed[key]
		return next, ok
	default:
		return nil, false
	}
}
