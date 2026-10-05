package authz

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrInvalidRule is returned by NewPolicy when a rule is malformed.
var ErrInvalidRule = errors.New("invalid rule")

// Rule matches requests and allows or denies them.
//
// A rule matches a request when the subject, the action and the resource each
// match at least one of the rule's patterns. A pattern is either:
//   - an exact value: "tools/call",
//   - "*", which matches any value,
//   - a prefix ending with "*": "mcp:github/*" matches "mcp:github/create_issue".
//
// "*" is only allowed as the last character of a pattern.
type Rule struct {
	ID        string
	Effect    Effect
	Subjects  []string
	Actions   []string
	Resources []string
}

// Policy is an immutable, validated set of rules.
type Policy struct {
	rules []Rule
}

// NewPolicy validates rules and builds a Policy.
// The rules are copied, so later changes to the input do not affect the Policy.
func NewPolicy(rules []Rule) (*Policy, error) {
	seen := make(map[string]struct{}, len(rules))
	copied := make([]Rule, 0, len(rules))

	for i, r := range rules {
		if err := r.validate(); err != nil {
			return nil, fmt.Errorf("%w: rule #%d (%q): %w", ErrInvalidRule, i, r.ID, err)
		}
		if _, dup := seen[r.ID]; dup {
			return nil, fmt.Errorf("%w: rule #%d: duplicate id %q", ErrInvalidRule, i, r.ID)
		}
		seen[r.ID] = struct{}{}

		r.Subjects = slices.Clone(r.Subjects)
		r.Actions = slices.Clone(r.Actions)
		r.Resources = slices.Clone(r.Resources)
		copied = append(copied, r)
	}

	return &Policy{rules: copied}, nil
}

// Evaluate decides whether req is allowed.
// Deny rules win over allow rules. With no matching rule, the request is denied.
// A nil Policy denies everything.
func (p *Policy) Evaluate(req Request) Decision {
	if p == nil {
		return Decision{Reason: ReasonNoPolicy}
	}
	if !req.valid() {
		return Decision{Reason: ReasonInvalidRequest}
	}

	var allow *Rule
	for i := range p.rules {
		r := &p.rules[i]
		if !r.matches(req) {
			continue
		}
		if r.Effect == EffectDeny {
			return Decision{RuleID: r.ID, Reason: ReasonDeniedByRule}
		}
		if allow == nil {
			allow = r
		}
	}

	if allow != nil {
		return Decision{Allowed: true, RuleID: allow.ID, Reason: ReasonAllowedByRule}
	}
	return Decision{Reason: ReasonNoMatchingRule}
}

func (r Rule) validate() error {
	if r.ID == "" {
		return errors.New("empty id")
	}
	if r.Effect != EffectAllow && r.Effect != EffectDeny {
		return fmt.Errorf("unknown effect %q", r.Effect)
	}
	for _, f := range []struct {
		name     string
		patterns []string
	}{
		{"subjects", r.Subjects},
		{"actions", r.Actions},
		{"resources", r.Resources},
	} {
		if len(f.patterns) == 0 {
			return fmt.Errorf("%s: at least one pattern is required", f.name)
		}
		for _, p := range f.patterns {
			if err := validatePattern(p); err != nil {
				return fmt.Errorf("%s: %w", f.name, err)
			}
		}
	}
	return nil
}

func (r Rule) matches(req Request) bool {
	return matchAny(r.Subjects, req.Subject) &&
		matchAny(r.Actions, req.Action) &&
		matchAny(r.Resources, req.Resource)
}

func validatePattern(p string) error {
	if p == "" {
		return errors.New("empty pattern")
	}
	if i := strings.IndexByte(p, '*'); i >= 0 && i != len(p)-1 {
		return fmt.Errorf("pattern %q: '*' is only allowed at the end", p)
	}
	return nil
}

func matchAny(patterns []string, value string) bool {
	return slices.ContainsFunc(patterns, func(p string) bool { return match(p, value) })
}

func match(pattern, value string) bool {
	if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
		return strings.HasPrefix(value, prefix)
	}
	return pattern == value
}
