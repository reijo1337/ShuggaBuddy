//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartFlow_NewUser(t *testing.T) {
	env := newTestEnv(t, false)

	env.sendStart()

	cnt := env.queryInt(`SELECT count(*) FROM users`)
	assert.Equal(t, int64(1), cnt)

	cnt = env.queryInt(`SELECT count(*) FROM external_accounts WHERE provider = 'telegram'`)
	assert.Equal(t, int64(1), cnt)

	require.Greater(t, env.bot.sentCount(), 0)
	msg := env.bot.lastMessageText()
	assert.NotEmpty(t, msg)
}

func TestStartFlow_ExistingUser(t *testing.T) {
	env := newTestEnv(t, true)

	env.bot.reset()
	env.sendStart()

	cnt := env.queryInt(`SELECT count(*) FROM users`)
	assert.Equal(t, int64(1), cnt)

	require.Greater(t, env.bot.sentCount(), 0)
}
