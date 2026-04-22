package telegram

// sessionType identifies which dialog flow a session belongs to.
type sessionType string

const (
	sessionGlucose   sessionType = "glucose"
	sessionFood      sessionType = "food"
	sessionCarbsUnit sessionType = "carbs_unit"
	sessionInsulin   sessionType = "insulin"
	sessionActivity  sessionType = "activity"
	sessionNote      sessionType = "note"
	sessionDiary     sessionType = "diary"
	sessionProfile   sessionType = "profile"
	sessionBolus     sessionType = "bolus"
	sessionAdvisor   sessionType = "advisor"
	sessionCGM       sessionType = "cgm"
)

// sessionStep identifies the current step within a session flow.
type sessionStep int

const (
	stepGlucoseValue sessionStep = iota

	stepFoodCarbs
	stepFoodNote
	stepFoodTime

	stepCarbsUnitValue

	stepInsulinType    // ожидание выбора типа инсулина (callback)
	stepInsulinDose    // ожидание ввода дозы текстом
	stepInsulinConfirm // ожидание подтверждения аномальной дозы (callback)
	stepInsulinDrug    // ожидание ввода препарата или пропуска

	stepActivityType      // ожидание выбора типа активности (callback)
	stepActivityCustom    // ожидание ввода произвольного типа текстом
	stepActivityDuration  // ожидание ввода длительности
	stepActivityTime      // ожидание выбора/ввода времени
	stepActivityIntensity // ожидание выбора интенсивности (callback)

	stepNoteWellbeing
	stepNoteText

	stepDiaryDate

	stepProfileTargetMin
	stepProfileTargetMax
	stepProfileBasalDrug
	stepProfileBasalTime

	stepBolusGlucose       // waiting for glucose confirmation or manual input
	stepBolusGlucoseManual // waiting for manual glucose text input
	stepBolusCarbs         // waiting for carbs text input

	stepProfileBasalDose // ожидание ввода дозы базального
	stepAdvisorInterval  // ожидание ввода произвольного интервала

	stepCGMURL   // ожидание ввода URL Nightscout
	stepCGMToken // ожидание ввода API secret/token

	stepLLUEmail    // ожидание ввода email LibreView
	stepLLUPassword // ожидание ввода пароля LibreView
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
