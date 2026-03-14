package telegram

// sessionType identifies which dialog flow a session belongs to.
type sessionType string

const (
	sessionGlucose   sessionType = "glucose"
	sessionFood      sessionType = "food"
	sessionCarbsUnit sessionType = "carbs_unit"
)

// sessionStep identifies the current step within a session flow.
type sessionStep int

const (
	stepGlucoseValue sessionStep = iota

	stepFoodCarbs
	stepFoodNote
	stepFoodTime

	stepCarbsUnitValue
)

// Session holds the state of an in-progress multi-step dialog for a chat.
// It is exported so that export_test.go can construct instances for white-box tests.
type Session struct {
	SType sessionType
	Step  sessionStep
	Data  map[string]any
}

// newSession creates a new session with an initialised data map.
func newSession(sType sessionType, step sessionStep) *Session {
	return &Session{SType: sType, Step: step, Data: make(map[string]any)}
}
