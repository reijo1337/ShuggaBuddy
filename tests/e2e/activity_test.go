//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivityFlow_Running(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("menu:activity")
	env.sendCallback("activity:type:running")
	env.sendCallback("activity:dur:30")
	env.sendCallback("activity:intensity:medium")
	env.sendCallback("activity:time:now")

	cnt := env.queryInt(
		`SELECT count(*) FROM activity_entries WHERE user_id = $1 AND activity_type = 'running'`,
		env.userID,
	)
	assert.Equal(t, int64(1), cnt)

	msg := env.bot.lastMessageText()
	require.NotEmpty(t, msg)
}

func TestActivityFlow_OtherType(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("menu:activity")
	env.sendCallback("activity:type:other")
	env.sendText("Скалолазание")
	env.sendCallback("activity:dur:45")
	env.sendCallback("activity:intensity:high")
	env.sendCallback("activity:time:now")

	cnt := env.queryInt(
		`SELECT count(*) FROM activity_entries WHERE user_id = $1 AND activity_type = 'other'`,
		env.userID,
	)
	assert.Equal(t, int64(1), cnt)
}
