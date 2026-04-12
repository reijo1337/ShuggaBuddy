//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBolusFlow_NoDrug(t *testing.T) {
	env := newTestEnv(t, true)

	// Try bolus without setting bolus drug — should get error.
	env.sendCallback("menu:insulin")
	env.sendCallback("insulin:type:bolus")

	msg := env.bot.lastMessageText()
	require.NotEmpty(t, msg)

	// No insulin_doses saved.
	cnt := env.queryInt(
		`SELECT count(*) FROM insulin_doses WHERE user_id = $1`,
		env.userID,
	)
	assert.Equal(t, int64(0), cnt)
}

func TestBolusFlow_InsufficientData(t *testing.T) {
	env := newTestEnv(t, true)

	// Set bolus drug first.
	env.sendCallback("profile:bolus_drug:set:novorapid")

	// Record a fresh glucose reading.
	env.sendCallback("menu:glucose")
	env.sendText("7.0")

	env.bot.reset()

	// Start bolus.
	env.sendCallback("bolus:start")

	// Confirm glucose.
	env.sendCallback("bolus:glucose:confirm")

	// Enter carbs.
	env.sendText("50")

	// Should get insufficient data response.
	msg := env.bot.lastMessageText()
	require.NotEmpty(t, msg)
}
