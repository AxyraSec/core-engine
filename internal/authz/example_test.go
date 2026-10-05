package authz_test

import (
	"fmt"

	"github.com/AxyraSec/core-engine/internal/authz"
)

func ExamplePolicy_Evaluate() {
	policy, err := authz.NewPolicy([]authz.Rule{
		{
			ID:        "allow-github-tools",
			Effect:    authz.EffectAllow,
			Subjects:  []string{"agent:billing-bot"},
			Actions:   []string{"tools/call"},
			Resources: []string{"mcp:github/*"},
		},
		{
			ID:        "deny-repo-delete",
			Effect:    authz.EffectDeny,
			Subjects:  []string{"*"},
			Actions:   []string{"tools/call"},
			Resources: []string{"mcp:github/delete_repo"},
		},
	})
	if err != nil {
		panic(err)
	}

	for _, tool := range []string{"create_issue", "delete_repo"} {
		d := policy.Evaluate(authz.Request{
			Subject:  "agent:billing-bot",
			Action:   "tools/call",
			Resource: "mcp:github/" + tool,
		})
		fmt.Println(tool, d.Allowed, d.Reason, d.RuleID)
	}

	// Output:
	// create_issue true allowed_by_rule allow-github-tools
	// delete_repo false denied_by_rule deny-repo-delete
}
