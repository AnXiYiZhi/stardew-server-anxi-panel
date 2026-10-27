package web

import (
	"io"
	"strings"
	"testing"
)

func TestJunimoSharedScopeConfirmationAcceptsOnlyExactAdminIntent(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, scenario := range []struct {
		name, body, scope string
		allowed           bool
	}{
		{"legacy confirm", `{"confirm":true}`, "", true},
		{"scoped confirm", `{"confirm":true,"sharedGroupSha256":"` + hash + `"}`, hash, true},
		{"missing confirm", `{"sharedGroupSha256":"` + hash + `"}`, "", false},
		{"false confirm", `{"confirm":false,"sharedGroupSha256":"` + hash + `"}`, "", false},
		{"duplicate confirm", `{"confirm":true,"confirm":true}`, "", false},
		{"case alias", `{"confirm":true,"Confirm":true}`, "", false},
		{"duplicate scope", `{"confirm":true,"sharedGroupSha256":"` + hash + `","sharedGroupSha256":"` + hash + `"}`, "", false},
		{"empty scope", `{"confirm":true,"sharedGroupSha256":""}`, "", false},
		{"null scope", `{"confirm":true,"sharedGroupSha256":null}`, "", false},
		{"nonhex scope", `{"confirm":true,"sharedGroupSha256":"` + strings.Repeat("g", 64) + `"}`, "", false},
		{"paths", `{"confirm":true,"members":["/data/other"]}`, "", false},
		{"image override", `{"confirm":true,"image":"other:1"}`, "", false},
		{"command", `{"confirm":true,"command":"shell"}`, "", false},
		{"trailing object", `{"confirm":true} {}`, "", false},
		{"oversize", `{"confirm":true}` + strings.Repeat(" ", 4096), "", false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			scope, allowed := acceptStrictJunimoScopeConfirmation(io.NopCloser(strings.NewReader(scenario.body)))
			if allowed != scenario.allowed || allowed && scope != scenario.scope {
				t.Fatalf("wrong scope authorization decision: allowed=%v scope=%q", allowed, scope)
			}
		})
	}
	if _, allowed := acceptStrictJunimoScopeConfirmation(nil); allowed {
		t.Fatal("absent body confirmed shared scope")
	}
}
