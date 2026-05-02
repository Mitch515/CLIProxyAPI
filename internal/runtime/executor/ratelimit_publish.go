package executor

import (
	"context"
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/ratelimit/parser"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// SelectedAuthContextKey is the context key under which the conductor stashes
// the *coreauth.Auth currently servicing a request. The string form mirrors
// the pattern used elsewhere (e.g. "cliproxy.roundtripper") so it is callable
// from any package without introducing import cycles.
const SelectedAuthContextKey = "cliproxy.selected_auth"

// publishRateLimitFromHeaders parses upstream rate-limit data and forwards
// observations onto the global ratelimit.Bus. Safe to call with empty
// headers and nil body. When no sink is registered (e.g. dashboard subsystem
// disabled) this becomes a cheap no-op.
//
// `body` should only be passed for non-2xx responses where rate-limit
// information lives in the JSON body (Codex usage cap, Gemini quota errors).
// For 2xx responses pass nil.
func publishRateLimitFromHeaders(ctx context.Context, status int, provider string, headers http.Header, body []byte) {
	if !ratelimit.HasSink() {
		return
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return
	}
	observations := parser.Parse(provider, status, headers, body)
	if len(observations) == 0 {
		return
	}
	auth, _ := SelectedAuthFromContext(ctx)
	authID, email, label := authIdentity(auth, provider)
	if authID == "" {
		// We have no idea which credential owned the request — drop. The
		// observation is useless without an account attribution.
		return
	}
	for _, o := range observations {
		o.AuthID = authID
		o.Provider = provider
		if o.Email == "" {
			o.Email = email
		}
		if o.Label == "" {
			o.Label = label
		}
		ratelimit.Submit(o)
	}
}

// SelectedAuthFromContext returns the auth stashed in ctx by the conductor.
// Returns (nil, false) when not present or wrong type.
func SelectedAuthFromContext(ctx context.Context) (*coreauth.Auth, bool) {
	if ctx == nil {
		return nil, false
	}
	v := ctx.Value(SelectedAuthContextKey)
	if v == nil {
		return nil, false
	}
	a, ok := v.(*coreauth.Auth)
	return a, ok
}

func authIdentity(a *coreauth.Auth, provider string) (id, email, label string) {
	if a == nil {
		return "", "", ""
	}
	id = a.ID
	label = a.Label
	if a.Metadata != nil {
		if v, ok := a.Metadata["email"].(string); ok {
			email = strings.TrimSpace(v)
		}
	}
	if email == "" {
		_, info := a.AccountInfo()
		email = info
	}
	return id, email, label
}
