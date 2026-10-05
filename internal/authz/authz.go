// Package authz decides whether a request from an AI agent is allowed.
//
// The model is deny by default:
//   - a request is allowed only if at least one allow rule matches it,
//   - a matching deny rule always wins over allow rules,
//   - an invalid request is always denied.
//
// A Policy is immutable once built, so it is safe for concurrent use.
package authz

// Effect is what a rule does when it matches a request.
type Effect string

// Rule effects.
const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// Request is a single action an agent wants to perform.
type Request struct {
	// Subject is the identity of the caller, for example "agent:billing-bot".
	Subject string
	// Action is the operation, for example "tools/call".
	Action string
	// Resource is the target, for example "mcp:github/create_issue".
	Resource string
}

// Decision is the result of evaluating a Request.
type Decision struct {
	// Allowed is true only when the request may proceed.
	Allowed bool
	// RuleID is the rule that decided. It is empty when no rule matched
	// or the request was invalid.
	RuleID string
	// Reason explains the decision in a stable, log-friendly form.
	Reason Reason
}

// Reason explains why a Decision was made.
type Reason string

// Decision reasons.
const (
	ReasonAllowedByRule  Reason = "allowed_by_rule"
	ReasonDeniedByRule   Reason = "denied_by_rule"
	ReasonNoMatchingRule Reason = "no_matching_rule"
	ReasonInvalidRequest Reason = "invalid_request"
	ReasonNoPolicy       Reason = "no_policy"
)

func (r Request) valid() bool {
	return r.Subject != "" && r.Action != "" && r.Resource != ""
}
