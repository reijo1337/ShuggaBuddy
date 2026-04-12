//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsulinFlow_Basal(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("menu:insulin")
	env.sendCallback("insulin:type:basal")
	env.sendText("10")
	env.sendCallback("insulin:skip_drug")

	cnt := env.queryInt(
		`SELECT count(*) FROM insulin_doses WHERE user_id = $1 AND insulin_type = 'basal'`,
		env.userID,
	)
	assert.Equal(t, int64(1), cnt)

	dose := env.queryFloat(
		`SELECT dose_units FROM insulin_doses WHERE user_id = $1`,
		env.userID,
	)
	assert.InDelta(t, 10.0, dose, 0.01)

	require.Greater(t, env.bot.sentCount(), 0)
}

func TestInsulinFlow_ManualBolus(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("insulin:manual")
	env.sendText("4")
	env.sendCallback("insulin:skip_drug")

	cnt := env.queryInt(
		`SELECT count(*) FROM insulin_doses WHERE user_id = $1 AND insulin_type = 'bolus'`,
		env.userID,
	)
	assert.Equal(t, int64(1), cnt)
}
