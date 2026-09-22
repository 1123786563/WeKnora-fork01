package openconnector

import (
	"errors"
	"strings"
)

// Grant is the token policy payload for the upstream runtime-token admin API
// (POST/PUT /api/runtime-tokens). Its JSON field names are frozen exactly as
// allowedConnections / allowedActions / allowedProxies / blockedActions: T01
// verified at runtime that a PUT must carry ALL FOUR arrays (a missing array
// is a 400) and that an extra field such as blockedProxies is rejected as
// invalid_input (docs/integrations/open-connector-contract.md §3.2).
type Grant struct {
	AllowedConnections []string `json:"allowedConnections"`
	AllowedActions     []string `json:"allowedActions"`
	AllowedProxies     []string `json:"allowedProxies"`
	BlockedActions     []string `json:"blockedActions"`
}

// Grant construction rejections. Both are fail-closed guards: the spec says
// "无连接或无允许动作时不签发 Token，绝不以空清单模拟拒绝全部".
var (
	// ErrEmptyGrant: no connection id or an empty action list.
	ErrEmptyGrant = errors.New("openconnector: empty grant")
	// ErrInvalidActionGrant: an action is blank or contains glob/path syntax.
	ErrInvalidActionGrant = errors.New("openconnector: invalid action grant")
)

// NewGrant builds the grant for one connection id and its allowed action ids.
// A blank connection id, a nil/empty action list, or any action containing
// glob ('*', '?') or path ('/') characters is rejected.
//
// WHY empty grants are rejected locally: upstream treats an EMPTY
// allowedConnections or allowedActions list as ALLOW-ALL — a token stored
// with [] may reach every connection and every action (runtime-proven,
// fixtures/empty_grant.json; contract doc §3.2). So an empty list can never
// stand in for "deny all" here: WeKnora refuses to mint such a token at all,
// and revoking the last authorization must DELETE the token instead of
// writing allowedConnections=[] (writing an empty list re-opens access).
//
// AllowedProxies is deliberately an explicit empty non-nil list: upstream
// empty allowedProxies means DENY-ALL proxies (the opposite asymmetry of the
// connection/action lists), which matches phase one — the provider proxy
// stays closed. Both empty lists marshal as [] because upstream PUT demands
// all four arrays present.
func NewGrant(id string, actions []string) (Grant, error) {
	if strings.TrimSpace(id) == "" || len(actions) == 0 {
		return Grant{}, ErrEmptyGrant
	}
	for _, a := range actions {
		if a == "" || strings.ContainsAny(a, "*?/") {
			return Grant{}, ErrInvalidActionGrant
		}
	}
	return Grant{
		AllowedConnections: []string{id},
		AllowedActions:     append([]string(nil), actions...),
		AllowedProxies:     []string{},
		BlockedActions:     []string{},
	}, nil
}
