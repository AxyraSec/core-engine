package authz

import (
	"errors"
	"testing"
)

func mustPolicy(t *testing.T, rules ...Rule) *Policy {
	t.Helper()
	p, err := NewPolicy(rules)
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}
	return p
}

func TestEvaluate(t *testing.T) {
	allowGitHub := Rule{
		ID:        "allow-github",
		Effect:    EffectAllow,
		Subjects:  []string{"agent:billing-bot"},
		Actions:   []string{"tools/call"},
		Resources: []string{"mcp:github/*"},
	}
	denyDelete := Rule{
		ID:        "deny-delete",
		Effect:    EffectDeny,
		Subjects:  []string{"*"},
		Actions:   []string{"tools/call"},
		Resources: []string{"mcp:github/delete_repo"},
	}
	allowAll := Rule{
		ID:        "allow-all",
		Effect:    EffectAllow,
		Subjects:  []string{"*"},
		Actions:   []string{"*"},
		Resources: []string{"*"},
	}

	createIssue := Request{Subject: "agent:billing-bot", Action: "tools/call", Resource: "mcp:github/create_issue"}
	deleteRepo := Request{Subject: "agent:billing-bot", Action: "tools/call", Resource: "mcp:github/delete_repo"}

	tests := []struct {
		name   string
		policy *Policy
		req    Request
		want   Decision
	}{
		{
			name:   "allowed by matching rule",
			policy: mustPolicy(t, allowGitHub),
			req:    createIssue,
			want:   Decision{Allowed: true, RuleID: "allow-github", Reason: ReasonAllowedByRule},
		},
		{
			name:   "empty policy denies",
			policy: mustPolicy(t),
			req:    createIssue,
			want:   Decision{Reason: ReasonNoMatchingRule},
		},
		{
			name:   "nil policy denies",
			policy: nil,
			req:    createIssue,
			want:   Decision{Reason: ReasonNoPolicy},
		},
		{
			name:   "other subject denied",
			policy: mustPolicy(t, allowGitHub),
			req:    Request{Subject: "agent:other", Action: "tools/call", Resource: "mcp:github/create_issue"},
			want:   Decision{Reason: ReasonNoMatchingRule},
		},
		{
			name:   "other action denied",
			policy: mustPolicy(t, allowGitHub),
			req:    Request{Subject: "agent:billing-bot", Action: "resources/read", Resource: "mcp:github/create_issue"},
			want:   Decision{Reason: ReasonNoMatchingRule},
		},
		{
			name:   "prefix does not match a sibling server",
			policy: mustPolicy(t, allowGitHub),
			req:    Request{Subject: "agent:billing-bot", Action: "tools/call", Resource: "mcp:github-enterprise/create_issue"},
			want:   Decision{Reason: ReasonNoMatchingRule},
		},
		{
			name:   "deny wins over earlier allow",
			policy: mustPolicy(t, allowGitHub, denyDelete),
			req:    deleteRepo,
			want:   Decision{RuleID: "deny-delete", Reason: ReasonDeniedByRule},
		},
		{
			name:   "deny wins over wildcard allow",
			policy: mustPolicy(t, allowAll, denyDelete),
			req:    deleteRepo,
			want:   Decision{RuleID: "deny-delete", Reason: ReasonDeniedByRule},
		},
		{
			name:   "first matching allow is reported",
			policy: mustPolicy(t, allowGitHub, allowAll),
			req:    createIssue,
			want:   Decision{Allowed: true, RuleID: "allow-github", Reason: ReasonAllowedByRule},
		},
		{
			name:   "empty subject is invalid",
			policy: mustPolicy(t, allowAll),
			req:    Request{Action: "tools/call", Resource: "mcp:github/create_issue"},
			want:   Decision{Reason: ReasonInvalidRequest},
		},
		{
			name:   "empty action is invalid",
			policy: mustPolicy(t, allowAll),
			req:    Request{Subject: "agent:billing-bot", Resource: "mcp:github/create_issue"},
			want:   Decision{Reason: ReasonInvalidRequest},
		},
		{
			name:   "empty resource is invalid",
			policy: mustPolicy(t, allowAll),
			req:    Request{Subject: "agent:billing-bot", Action: "tools/call"},
			want:   Decision{Reason: ReasonInvalidRequest},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.policy.Evaluate(tt.req); got != tt.want {
				t.Errorf("Evaluate() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNewPolicyRejectsInvalidRules(t *testing.T) {
	valid := func() Rule {
		return Rule{
			ID:        "r1",
			Effect:    EffectAllow,
			Subjects:  []string{"*"},
			Actions:   []string{"*"},
			Resources: []string{"*"},
		}
	}

	tests := []struct {
		name   string
		mutate func(*Rule)
	}{
		{"empty id", func(r *Rule) { r.ID = "" }},
		{"empty effect", func(r *Rule) { r.Effect = "" }},
		{"unknown effect", func(r *Rule) { r.Effect = "Allow" }},
		{"no subjects", func(r *Rule) { r.Subjects = nil }},
		{"no actions", func(r *Rule) { r.Actions = []string{} }},
		{"no resources", func(r *Rule) { r.Resources = nil }},
		{"empty pattern", func(r *Rule) { r.Subjects = []string{""} }},
		{"wildcard in the middle", func(r *Rule) { r.Resources = []string{"mcp:*/delete_repo"} }},
		{"two wildcards", func(r *Rule) { r.Actions = []string{"**"} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := valid()
			tt.mutate(&r)
			p, err := NewPolicy([]Rule{r})
			if !errors.Is(err, ErrInvalidRule) {
				t.Fatalf("NewPolicy() error = %v, want ErrInvalidRule", err)
			}
			if p != nil {
				t.Errorf("NewPolicy() returned a policy with an error")
			}
		})
	}
}

func TestNewPolicyRejectsDuplicateIDs(t *testing.T) {
	r := Rule{ID: "dup", Effect: EffectAllow, Subjects: []string{"*"}, Actions: []string{"*"}, Resources: []string{"*"}}
	if _, err := NewPolicy([]Rule{r, r}); !errors.Is(err, ErrInvalidRule) {
		t.Fatalf("NewPolicy() error = %v, want ErrInvalidRule", err)
	}
}

func TestPolicyIsNotAffectedByInputChanges(t *testing.T) {
	rules := []Rule{{
		ID:        "allow-read",
		Effect:    EffectAllow,
		Subjects:  []string{"agent:a"},
		Actions:   []string{"resources/read"},
		Resources: []string{"doc:1"},
	}}
	p := mustPolicy(t, rules...)

	// Widen the rule after the policy was built.
	rules[0].Resources[0] = "*"

	got := p.Evaluate(Request{Subject: "agent:a", Action: "resources/read", Resource: "doc:2"})
	if got.Allowed {
		t.Fatalf("policy changed after build: %+v", got)
	}
}

// FuzzEvaluateDeniesWithoutAllowRule checks that a policy with only deny rules
// never allows anything, whatever the request.
func FuzzEvaluateDeniesWithoutAllowRule(f *testing.F) {
	f.Add("agent:a", "tools/call", "mcp:github/delete_repo")
	f.Add("", "", "")
	f.Add("*", "*", "*")

	p, err := NewPolicy([]Rule{{
		ID:        "deny-github",
		Effect:    EffectDeny,
		Subjects:  []string{"*"},
		Actions:   []string{"*"},
		Resources: []string{"mcp:github/*"},
	}})
	if err != nil {
		f.Fatal(err)
	}

	f.Fuzz(func(t *testing.T, subject, action, resource string) {
		if d := p.Evaluate(Request{Subject: subject, Action: action, Resource: resource}); d.Allowed {
			t.Fatalf("allowed without any allow rule: %+v", d)
		}
	})
}
