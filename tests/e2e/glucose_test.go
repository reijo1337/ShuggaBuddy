//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlucoseFlow_InRange(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("menu:glucose")
	env.sendText("5.4")

	val := env.queryFloat(
		`SELECT value_mmol FROM glucose_readings WHERE user_id = $1`,
		env.userID,
	)
	assert.InDelta(t, 5.4, val, 0.01)

	msg := env.bot.lastMessageText()
	assert.Contains(t, msg, "5.4")
	assert.Contains(t, msg, "🟢")
}

func TestGlucoseFlow_High(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("menu:glucose")
	env.sendText("12.5")

	val := env.queryFloat(
		`SELECT value_mmol FROM glucose_readings WHERE user_id = $1`,
		env.userID,
	)
	assert.InDelta(t, 12.5, val, 0.01)

	msg := env.bot.lastMessageText()
	assert.Contains(t, msg, "12.5")
	assert.Contains(t, msg, "🟡")
}

func TestGlucoseFlow_InvalidInput(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("menu:glucose")
	env.sendText("abc")

	cnt := env.queryInt(
		`SELECT count(*) FROM glucose_readings WHERE user_id = $1`,
		env.userID,
	)
	assert.Equal(t, int64(0), cnt)

	require.Greater(t, env.bot.sentCount(), 0)
}
