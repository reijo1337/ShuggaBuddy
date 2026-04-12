//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoteFlow_Wellbeing(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("menu:note")
	env.sendCallback("note:type:wellbeing")
	env.sendCallback("note:wellbeing:good")
	env.sendCallback("note:skip")

	cnt := env.queryInt(
		`SELECT count(*) FROM notes WHERE user_id = $1 AND type = 'wellbeing'`,
		env.userID,
	)
	assert.Equal(t, int64(1), cnt)

	require.Greater(t, env.bot.sentCount(), 0)
}

func TestNoteFlow_FreeText(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("menu:note")
	env.sendCallback("note:type:free")
	env.sendText("Голова болит после обеда")

	cnt := env.queryInt(
		`SELECT count(*) FROM notes WHERE user_id = $1 AND type = 'free'`,
		env.userID,
	)
	assert.Equal(t, int64(1), cnt)
}
