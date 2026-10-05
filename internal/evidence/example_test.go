package evidence_test

import (
	"fmt"

	"github.com/AxyraSec/core-engine/internal/evidence"
)

func ExampleVerify() {
	chain := evidence.NewChain()
	chain.Append(evidence.Entry{
		Subject: "agent:billing-bot", Action: "tools/call", Resource: "mcp:github/create_issue",
		Allowed: true, RuleID: "allow-github-tools", Reason: "allowed_by_rule",
	})
	chain.Append(evidence.Entry{
		Subject: "agent:billing-bot", Action: "tools/call", Resource: "mcp:github/delete_repo",
		RuleID: "deny-repo-delete", Reason: "denied_by_rule",
	})

	records := chain.Records()
	fmt.Println("untouched:", evidence.Verify(records))

	// Someone rewrites history to hide the denied call.
	records[1].Allowed = true
	fmt.Println("tampered:", evidence.Verify(records))

	// Output:
	// untouched: <nil>
	// tampered: evidence chain is broken: record #1: content does not match its hash
}
