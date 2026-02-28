package shared

import (
	"encoding/json"
	"os"
	"strings"
)

type McpServerConfig struct {
	// for stdio
	Command string            `json:"command" yaml:"command"`
	Args    []string          `json:"args" yaml:"args"`
	Env     map[string]string `json:"env" yaml:"env"`
	// for http
	Type    string            `json:"type" yaml:"type"`
	Url     string            `json:"url" yaml:"url"`
	Headers map[string]string `json:"headers" yaml:"headers"`
}

func (s *McpServerConfig) IsStdio() bool {
	return s.Command != ""
}

func (s *McpServerConfig) IsHttp() bool {
	return s.Url != ""
}

func (s *McpServerConfig) ReplacePlaceholders(replaceMap map[string]string) McpServerConfig {
	newConfig := McpServerConfig{
		Command: s.Command,
		Args:    make([]string, 0),
		Env:     s.Env,
		Type:    s.Type,
		Url:     s.Url,
		Headers: s.Headers,
	}

	for _, arg := range s.Args {
		newArg := arg
		for k, v := range replaceMap {
			newArg = strings.ReplaceAll(newArg, k, v)
		}
		newConfig.Args = append(newConfig.Args, newArg)
	}

	return newConfig
}

func LoadMcpServerConfig(path string) (map[string]McpServerConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	serverMap := make(map[string]McpServerConfig)

	err = json.Unmarshal(content, &serverMap)
	if err != nil {
		return nil, err
	}
	return serverMap, nil
}
