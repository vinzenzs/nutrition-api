package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/agenttools"
)

// The chat loop exposes the shared coach registry and adds the web_search
// server tool. The detailed schema/build/tier assertions live in the
// agenttools package; here we verify the chat-side rendering, that the
// consequential coach writes are present but gated write-confirm, and that
// genuinely out-of-surface tools are still absent.
func TestChatToolDefs_SurfaceAndWebSearch(t *testing.T) {
	// The chat coach surface is the chat-exposed subset of the shared registry;
	// MCP-only tools (e.g. garmin_login) live in the union but not here.
	specs := agenttools.ChatRegistry()
	byName := agenttools.ByName(specs)

	// The coach write actions are now in-surface — but write-confirm (gated),
	// not write-auto. (Confirms task 3.3: out-of-surface writes are now
	// in-surface but require confirmation.)
	for _, n := range []string{"log_workout", "delete_workout", "set_daily_goal_override", "log_meal_freeform"} {
		s, ok := byName[n]
		require.Truef(t, ok, "coach tool %s should be in surface", n)
		assert.Equalf(t, agenttools.TierWriteConfirm, s.Tier, "%s must be gated write-confirm", n)
	}
	// The aggregate grounding reads are present.
	for _, n := range []string{"get_training_context", "get_recovery_context"} {
		assert.Contains(t, byName, n)
	}
	// Tools with no endpoint in this app must still be absent.
	for _, f := range []string{"delete_everything", "send_email", "garmin_login"} {
		assert.NotContainsf(t, byName, f, "unexpected tool present: %s", f)
	}

	// Tool defs include web_search, domain-restricted to Cookidoo.
	defs := anthropicToolDefs(specs)
	require.Len(t, defs, len(specs)+1) // custom tools + web_search
	last := string(defs[len(defs)-1])
	assert.Contains(t, last, "web_search")
	assert.Contains(t, last, "cookidoo.de")
	assert.Contains(t, last, "allowed_domains")
}
