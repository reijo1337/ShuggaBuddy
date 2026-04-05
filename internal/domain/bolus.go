package domain

// InsulinProfile describes the pharmacokinetic properties of a bolus insulin.
type InsulinProfile struct {
	Name    string
	DIA     float64 // duration of insulin action in hours
	PeakMin float64 // start of peak activity in hours
	PeakMax float64 // end of peak activity in hours
}

// BolusInsulinCatalog maps drug keys to their profiles.
var BolusInsulinCatalog = map[string]InsulinProfile{
	"fiasp":      {Name: "Fiasp", DIA: 3.5, PeakMin: 0.5, PeakMax: 1.0},
	"lyumjev":    {Name: "Lyumjev", DIA: 3.5, PeakMin: 0.5, PeakMax: 1.0},
	"novorapid":  {Name: "NovoRapid", DIA: 4.0, PeakMin: 1.0, PeakMax: 2.0},
	"humalog":    {Name: "Humalog", DIA: 4.0, PeakMin: 1.0, PeakMax: 2.0},
	"apidra":     {Name: "Apidra", DIA: 4.0, PeakMin: 1.0, PeakMax: 1.5},
	"rosinsulin": {Name: "Росинсулин Аспарт", DIA: 4.0, PeakMin: 1.0, PeakMax: 2.0},
	"actrapid":   {Name: "Actrapid", DIA: 5.0, PeakMin: 2.0, PeakMax: 3.0},
	"humulin_r":  {Name: "Humulin R", DIA: 5.0, PeakMin: 2.0, PeakMax: 3.0},
}

// BolusRecommendation holds the result of a bolus calculation.
type BolusRecommendation struct {
	FoodDose       float64 // units for carbs coverage
	CorrectionDose float64 // units for glucose correction
	IOB            float64 // insulin on board (subtracted)
	TotalDose      float64 // final recommendation (>= 0)
	ICR            float64 // derived insulin-to-carb ratio (g per 1 unit)
	ISF            float64 // derived insulin sensitivity factor (mmol/L per 1 unit)
	CurrentGlucose float64
	TargetGlucose  float64 // midpoint of target range
	CarbsGrams     float64
}
