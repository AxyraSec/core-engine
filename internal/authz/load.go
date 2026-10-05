package authz

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// MaxPolicySize is the largest policy file LoadPolicy accepts, in bytes.
const MaxPolicySize = 1 << 20 // 1 MiB

// ErrInvalidPolicyFile is returned when a policy file cannot be parsed.
// Errors from rule validation wrap ErrInvalidRule instead.
var ErrInvalidPolicyFile = errors.New("invalid policy file")

// policyFile is the YAML schema of a policy file.
type policyFile struct {
	Version int        `yaml:"version"`
	Rules   []ruleFile `yaml:"rules"`
}

type ruleFile struct {
	ID        string   `yaml:"id"`
	Effect    string   `yaml:"effect"`
	Subjects  []string `yaml:"subjects"`
	Actions   []string `yaml:"actions"`
	Resources []string `yaml:"resources"`
}

// LoadPolicyFile reads and validates the YAML policy file at path.
func LoadPolicyFile(path string) (*Policy, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	p, err := LoadPolicy(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

// LoadPolicy reads and validates a YAML policy.
//
// The input must hold exactly one YAML document, be at most MaxPolicySize
// bytes, use "version: 1", and contain only known fields. Duplicate keys are
// rejected. Nothing is returned unless the whole policy is valid.
func LoadPolicy(r io.Reader) (*Policy, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxPolicySize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxPolicySize {
		return nil, fmt.Errorf("%w: larger than %d bytes", ErrInvalidPolicyFile, MaxPolicySize)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var pf policyFile
	if err := dec.Decode(&pf); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w: empty", ErrInvalidPolicyFile)
		}
		return nil, fmt.Errorf("%w: %w", ErrInvalidPolicyFile, err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: more than one YAML document", ErrInvalidPolicyFile)
	}
	if pf.Version != 1 {
		return nil, fmt.Errorf("%w: unsupported version %d, want 1", ErrInvalidPolicyFile, pf.Version)
	}

	rules := make([]Rule, 0, len(pf.Rules))
	for _, fr := range pf.Rules {
		rules = append(rules, Rule{
			ID:        fr.ID,
			Effect:    Effect(fr.Effect),
			Subjects:  fr.Subjects,
			Actions:   fr.Actions,
			Resources: fr.Resources,
		})
	}
	return NewPolicy(rules)
}
