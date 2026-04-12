//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvisorFlow_ShowAnalysis(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("menu:advisor")

	msg := env.bot.lastMessageText()
	require.NotEmpty(t, msg)
}

func TestAdvisorFlow_SetInterval(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("profile:advisor_interval")
	env.sendCallback("profile:advisor_interval:7")

	days := env.queryInt(
		`SELECT advisor_interval_days FROM users WHERE id = $1`,
		env.userID,
	)
	assert.Equal(t, int64(7), days)
}

func TestAdvisorFlow_TurnOff(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("profile:advisor_interval")
	env.sendCallback("profile:advisor_interval:7")

	env.sendCallback("profile:advisor_interval")
	env.sendCallback("profile:advisor_interval:off")

	days := env.queryInt(
		`SELECT advisor_interval_days FROM users WHERE id = $1`,
		env.userID,
	)
	assert.Equal(t, int64(0), days)
}
