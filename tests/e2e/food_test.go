//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFoodFlow_WithNote(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("menu:food")
	env.sendText("60")
	env.sendText("Обед")
	env.sendCallback("food:time:now")

	carbs := env.queryFloat(
		`SELECT carbs_grams FROM food_entries WHERE user_id = $1`,
		env.userID,
	)
	assert.InDelta(t, 60.0, carbs, 0.01)

	require.Greater(t, env.bot.sentCount(), 0)
}

func TestFoodFlow_SkipNote(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("menu:food")
	env.sendText("30")
	env.sendCallback("food:skip_note")
	env.sendCallback("food:time:-15")

	cnt := env.queryInt(
		`SELECT count(*) FROM food_entries WHERE user_id = $1`,
		env.userID,
	)
	assert.Equal(t, int64(1), cnt)
}
