package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The assistant is named Kazper (rebrand-to-kazper): the server-assembled
// system prompt introduces the coach by name, and the config-injected diet /
// timezone are folded in.
func TestBuildSystemPrompt_NamesKazper(t *testing.T) {
	got := buildSystemPrompt(promptParams{DietaryPreferences: "vegetarian", Timezone: "Europe/Berlin"})
	assert.Contains(t, got, "You are Kazper,", "the coach should introduce itself as Kazper")
	assert.Contains(t, got, "vegetarian", "dietary preference should be injected")
	assert.Contains(t, got, "Europe/Berlin", "timezone should be injected")
}
