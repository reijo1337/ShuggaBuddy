//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileFlow_SetUnits(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("menu:profile")
	env.sendCallback("menu:units")
	env.sendCallback("units:mgdl")

	var units string
	err := env.pool.QueryRow(env.ctx,
		`SELECT units FROM users WHERE id = $1`, env.userID,
	).Scan(&units)
	require.NoError(t, err)
	assert.Equal(t, "mgdl", units)
}

func TestProfileFlow_TargetRange(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("profile:target_range")
	env.sendText("4.0")
	env.sendText("8.0")

	var minVal, maxVal float64
	err := env.pool.QueryRow(env.ctx,
		`SELECT target_min_mmol, target_max_mmol FROM users WHERE id = $1`,
		env.userID,
	).Scan(&minVal, &maxVal)
	require.NoError(t, err)
	assert.InDelta(t, 4.0, minVal, 0.01)
	assert.InDelta(t, 8.0, maxVal, 0.01)
}

func TestProfileFlow_BasalSettings(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("profile:basal")
	env.sendText("Тресиба")
	env.sendText("22:00")

	var drug, basalTime string
	err := env.pool.QueryRow(env.ctx,
		`SELECT basal_drug, basal_time FROM users WHERE id = $1`,
		env.userID,
	).Scan(&drug, &basalTime)
	require.NoError(t, err)
	assert.Equal(t, "Тресиба", drug)
	assert.Equal(t, "22:00", basalTime)
}

func TestProfileFlow_BasalDose(t *testing.T) {
	env := newTestEnv(t, true)

	env.sendCallback("profile:basal_dose")
	env.sendText("14")

	dose := env.queryFloat(
		`SELECT basal_dose FROM users WHERE id = $1`,
		env.userID,
	)
	assert.InDelta(t, 14.0, dose, 0.01)
}
