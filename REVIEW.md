# Code Review: Bolus Calculator Feature

**Reviewer:** Code Review Agent
**Date:** 2026-04-05
**Branch:** main (staged changes on top of 69a0c9d)
**Status:** All tests pass (bolus usecase, delivery telegram, user usecase)

---

## Overall Assessment

The bolus calculator feature is well-implemented and follows the project's Clean Architecture conventions. The domain model is clean, the usecase logic is mathematically sound, the delivery layer properly delegates to usecases, and test coverage is solid. The code integrates naturally into the existing insulin recording flow. Below are specific findings organized by severity.

---

## Critical Issues

### C1. Saved bolus injection does not set `source=calculator`

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/delivery/telegram/bolus.go`, line 244
**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/usecase/insulin/insulin_usecase.go`, line 40

**Description:** The CLAUDE.md plan explicitly requires: "Record button saves as injection with source=calculator." However, `handleBolusSave` calls `h.insulinUC.SaveDose(...)` which creates an `InsulinDose` with `Source` left as the zero value (empty string). The `insulin_doses.source` column defaults to `'manual'` in the database, so all calculator-originated doses will be indistinguishable from manual ones.

This is a functional bug: the `source` field was added in migration 013 specifically for this purpose, yet it is never set to `"bolus_calculator"`.

**Suggested fix:** Either:
- (a) Add a `source` parameter to `InsulinUseCase.SaveDose` interface (breaking change, requires updating all callers), or
- (b) Add a separate method like `SaveDoseWithSource(ctx, userID, dose, insulinType, drug, source string) error` to the `InsulinUseCase` interface and use it from `handleBolusSave`, or
- (c) Minimally, add `Source string` to the existing `SaveDose` and default it to `"manual"` for existing call sites.

Option (b) is recommended as least disruptive. Add the interface method to `deps.go`, implement in `internal/usecase/insulin/insulin_usecase.go`, update `handleBolusSave` to call it with `"bolus_calculator"`, and regenerate mocks.

---

### C2. Glucose input validation in calculator only accepts mmol/L range

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/delivery/telegram/bolus.go`, lines 143-146

**Description:** `handleBolusGlucoseInput` validates the manual glucose entry against a hardcoded range of `1.0-33.3`, which is the mmol/L range. Users who have their units set to mg/dL would need to enter values in the 18-600 range, but those values would be rejected by this validation. Additionally, the value is stored and passed to the calculator in the entered unit without conversion to mmol/L.

The existing glucose recording flow in the bot handles unit conversion properly. The calculator should do the same: check the user's preferred units, validate in those units, and convert to mmol/L before passing to the usecase.

**Suggested fix:** In `handleBolusStart`, store the user's units in the session data. In `handleBolusGlucoseInput`, check those units: if mg/dL, validate against 18-600 and convert to mmol/L before storing in `sess.Data["glucose"]`. Alternatively, since the calculator always needs mmol/L internally, always prompt and accept mmol/L but clearly label the prompt.

---

## Important Issues

### I1. `deriveICR` uses first-match for meal-bolus pairing, not closest

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/usecase/bolus/bolus_usecase.go`, lines 76-89

**Description:** When matching a food entry to a bolus dose, the function takes the first dose that falls within the `MealBolusWindowMin` window (`break` on line 88). If the data is not sorted by proximity to the food entry, this could match a less-relevant dose. The `doses` slice comes from the repository sorted by `recorded_at DESC`, meaning the most recent dose is first. For food entries that are also recent, this works, but for earlier food entries it may match an incorrect dose.

This is a data quality concern rather than a crash bug. The median calculation provides some resilience against outliers, but incorrect pairing could still skew the ICR.

**Suggested fix:** Instead of breaking on the first match, find the dose with the minimum absolute time gap within the window:

```go
var matchedDose *domain.InsulinDose
var bestGap time.Duration
for i := range doses {
    if doses[i].InsulinType != domain.InsulinTypeBolus {
        continue
    }
    gap := doses[i].RecordedAt.Sub(food.EatenAt)
    if gap < 0 {
        gap = -gap
    }
    if gap <= mealWindow && (matchedDose == nil || gap < bestGap) {
        matchedDose = &doses[i]
        bestGap = gap
    }
}
```

### I2. `deriveISF` also uses first-match for post-bolus glucose

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/usecase/bolus/bolus_usecase.go`, lines 167-174

**Description:** Similar to I1, the function takes the first glucose reading in the post-bolus window rather than the closest to the target time (e.g., midpoint of the 2-4h window). Since glucose readings are sorted DESC from the repository, the first match within the window is the most recent one in that range, which is acceptable but not optimal. Consider using the reading closest to the 3-hour mark for more consistent ISF estimation.

**Suggested fix:** Low priority, but consider selecting the reading closest to 3 hours post-bolus rather than the first found.

### I3. `handleBolusCarbsInput` uses `time.Now()` directly -- not testable

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/delivery/telegram/bolus.go`, line 167

**Description:** The call `h.bolusUC.Calculate(ctx, userID, glucose, carbs, time.Now())` makes the `now` parameter non-deterministic in tests. The test in `bolus_test.go` line 145 uses `gomock.Any()` for the `now` parameter, which hides potential time-related bugs. This is consistent with how other handlers work in the project (e.g., diary uses `time.Now()` directly), so it is a project-wide pattern, but worth noting.

**Suggested fix:** Accept as a project convention, or if improved testability is desired, consider injecting a clock interface.

### I4. Missing validation for `drugKey` in `handleBolusDrugSet` before catalog lookup

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/delivery/telegram/profile.go`, line 277

**Description:** After `UpdateBolusDrug` succeeds, line 277 does `profile := domain.BolusInsulinCatalog[drugKey]` without checking the `ok` value. If someone crafted a callback with an invalid `drugKey`, `UpdateBolusDrug` in the usecase would reject it (line 128 of user_usecase.go validates against catalog), so the code would never reach line 277 with an invalid key. However, as a defensive practice, the map lookup should check `ok`.

**Suggested fix:** The usecase validation makes this safe in practice. For defense-in-depth, add:
```go
profile, ok := domain.BolusInsulinCatalog[drugKey]
if !ok {
    h.reply(cb.Message.Chat.ID, h.loc.T("error_internal"))
    return
}
```

### I5. `formatDuration` hardcodes Russian text outside of i18n

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/delivery/telegram/bolus.go`, lines 255-261

**Description:** The function returns hardcoded Russian strings ("только что", "%d мин") instead of using the `i18n.Localizer`. While the project currently supports only Russian, the CLAUDE.md states the architecture is ready for multi-language support. All user-facing strings should go through the localizer per project convention.

**Suggested fix:** Add localization keys like `duration_just_now` and `duration_minutes` to `locales/ru.yaml` and use `h.loc.T(...)` instead. Note that `formatDuration` is a standalone function, not a method on Handler, so it would need to be refactored to accept the localizer or be converted to a method.

---

## Minor Issues / Suggestions

### S1. `insulin:manual` callback creates a bolus-type session without the bolus method choice

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/delivery/telegram/user.go`, lines 155-158

**Description:** When the user chooses "Enter dose manually" from the bolus method choice screen, the callback `insulin:manual` creates a new insulin session with type hardcoded to `domain.InsulinTypeBolus`. This is correct behavior (the user chose bolus type, then manual entry), but the session is created fresh without inheriting anything from the previous flow. If the previous insulin session had additional state, it would be lost. In the current implementation this is fine since no extra state exists at that point.

**Action:** No fix needed, just documenting for awareness.

### S2. `GlucoseRepository.GetByTimeRange` returns `[]GlucoseReading` (value), while other repos return `[]*Type` (pointer)

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/domain/glucose.go`, line 22 vs `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/domain/insulin.go`, line 33

**Description:** `GlucoseRepository.GetByTimeRange` returns `[]domain.GlucoseReading` while `InsulinRepository.GetByTimeRange` returns `[]*domain.InsulinDose` and `FoodRepository.GetByTimeRange` returns `[]*domain.FoodEntry`. This inconsistency existed before this feature but the bolus usecase makes it more visible since it consumes all three. The bolus usecase correctly handles this by dereferencing pointer slices on lines 240-248 of `bolus_usecase.go`.

**Action:** Not in scope for this feature, but consider standardizing in a future cleanup.

### S3. Test for `handleBolusDetails` callback is missing

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/delivery/telegram/bolus_test.go`

**Description:** There is no test for the `bolus:details` callback (`handleBolusDetails`). The handler reads the recommendation from the session and formats a detailed breakdown. A test would ensure the details message is correctly formatted.

**Suggested fix:** Add a test case similar to `TestBolusSave_Success` that stores a recommendation in the session and triggers the `bolus:details` callback, then verifies the response contains the expected breakdown fields (ICR, ISF, IOB, etc.).

### S4. Test for `bolus:cancel` callback is missing

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/delivery/telegram/bolus_test.go`

**Description:** The `bolus:cancel` callback clears the session and returns to menu. No test covers this path.

**Suggested fix:** Add a simple test that sets up a bolus session, triggers `bolus:cancel`, and verifies the session was cleared and the menu was shown.

### S5. Median function does not handle empty slice

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/usecase/bolus/bolus_usecase.go`, lines 195-202

**Description:** `median(values []float64)` will panic on an empty slice (index out of range). Currently, callers (`deriveICR` and `deriveISF`) always check `len(ratios) < MinChains` before calling `median`, so an empty slice never reaches `median`. This is safe but fragile.

**Suggested fix:** Add a guard at the top of `median`:
```go
if len(values) == 0 {
    return 0
}
```

### S6. `BolusInsulinCatalog` is a mutable global variable

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/domain/bolus.go`, line 12

**Description:** `BolusInsulinCatalog` is a `var` (mutable map). Any code could accidentally modify it at runtime. Since it is a static reference table, it would be safer as an immutable lookup.

**Suggested fix:** Change to a function `GetBolusInsulinProfile(key string) (InsulinProfile, bool)` that returns values from an unexported map, or accept the current approach given the small codebase. Not blocking.

### S7. `handleBolusGlucoseInput` does not validate maximum dose for high glucose

**File:** `/Users/gmtantsevov/petproject/ShuggaBuddy/internal/delivery/telegram/bolus.go`, line 144

**Description:** The validation `value > 33.3` would reject a reading of, say, 33.4 mmol/L. While this matches the glucose validation range in the spec (1.0-33.3), in the context of a bolus calculator, a user with an extremely high reading might want to use the calculator. This is consistent with the existing glucose recording flow, so no change needed, but worth noting for product consideration.

**Action:** No code change needed -- consistent with existing behavior.

---

## Plan Alignment Summary

| Requirement | Status | Notes |
|---|---|---|
| ICR from meal-bolus-glucose chains | DONE | Median of ratios from matching chains |
| ISF from correction boluses | DONE | Median of drop/dose from correction-only boluses |
| IOB with linear decay | DONE | Linear model, filters by DIA window |
| DIA from BolusInsulinCatalog | DONE | 8 drugs with pharmacokinetic profiles |
| Bolus drug setup in profile | DONE | Selection menu with sorted catalog |
| Integrated into insulin flow | DONE | Type selection -> "Manual" / "Calculator" split |
| Suggest last glucose if < 30 min | DONE | Confirmation + manual entry buttons |
| Brief result + Details button | DONE | Summary with expandable details |
| Record button saves injection | PARTIAL | Saves injection but **source is not set to "calculator"** (see C1) |
| Min 5 chains for recommendation | DONE | Error returned when < 5 chains found |
| source=calculator on saved dose | NOT DONE | See Critical issue C1 |
| mg/dL glucose support in calculator | NOT DONE | See Critical issue C2 |

---

## Architecture Compliance

- **Clean Architecture layers:** Correctly followed. `delivery -> usecase -> domain <- repository`. No reverse dependencies detected.
- **Domain purity:** `internal/domain/bolus.go` has zero external dependencies. Correct.
- **Repository layer:** Only CRUD, no business logic. `GetByTimeRange` added cleanly.
- **Delivery layer:** Uses interfaces from `deps.go`, never calls repositories directly. Correct.
- **i18n:** All user-facing strings go through `h.loc.T(...)` except `formatDuration` (see I5).
- **Testing:** Both usecase and delivery layers have tests. Mocks properly generated.
- **Migration:** Correct format with `+goose Up` / `+goose Down`. Down migration properly reverses changes.

---

## What Was Done Well

1. **Clean separation of calculation logic** -- `calculateIOB`, `deriveICR`, `deriveISF` are pure functions with no side effects, making them easy to test and reason about.
2. **Comprehensive usecase tests** -- IOB, ICR, ISF each tested individually with multiple scenarios, plus integration-level `Calculate` tests.
3. **Proper error propagation** -- All errors are wrapped with context (`fmt.Errorf("bolus.Calculate: %w", err)`).
4. **Graceful degradation** -- When insufficient data, the user gets a clear message about needing more diary entries.
5. **Session management** -- Bolus flow properly cleans up sessions on completion and cancellation.
6. **Profile integration** -- Bolus drug selection seamlessly added to profile view with sorted catalog display.
7. **Migration is reversible** -- Down migration correctly drops both added columns.
8. **Delivery tests cover key flows** -- No drug set, fresh glucose, no fresh glucose, insufficient data, save success.

---

## Action Items (prioritized)

1. **[CRITICAL]** Fix C1: Add `source=calculator` when saving bolus-calculator doses
2. **[CRITICAL]** Fix C2: Handle mg/dL users in glucose input for calculator
3. **[IMPORTANT]** Fix I1: Use closest-match instead of first-match in `deriveICR`
4. **[IMPORTANT]** Fix I5: Move `formatDuration` strings to i18n
5. **[MINOR]** Add S3: Test for `bolus:details` callback
6. **[MINOR]** Add S4: Test for `bolus:cancel` callback
7. **[MINOR]** Fix S5: Guard `median()` against empty slice
8. **[MINOR]** Fix I4: Defensive check on catalog lookup in `handleBolusDrugSet`
