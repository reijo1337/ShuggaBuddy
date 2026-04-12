//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiaryFlow_WithEntries(t *testing.T) {
	env := newTestEnv(t, true)

	// Seed: record glucose.
	env.sendCallback("menu:glucose")
	env.sendText("6.2")

	// Seed: record food.
	env.sendCallback("menu:food")
	env.sendText("45")
	env.sendCallback("food:skip_note")
	env.sendCallback("food:time:now")

	env.bot.reset()

	// Open diary (shows today).
	env.sendCallback("menu:diary")

	msg := env.bot.lastMessageText()
	require.NotEmpty(t, msg)

	assert.Contains(t, msg, "6.2")
	assert.Contains(t, msg, "45")
}

func TestDiaryFlow_Empty(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("menu:diary")

	msg := env.bot.lastMessageText()
	require.NotEmpty(t, msg)
}
