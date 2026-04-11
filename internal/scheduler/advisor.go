package scheduler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/i18n"
)

// AdvisorAnalyzer is the interface for dose analysis.
type AdvisorAnalyzer interface {
	Analyze(ctx context.Context, userID int64, now time.Time) (*domain.DoseAdvice, error)
}

// AdvisorScheduler sends periodic dose recommendations.
type AdvisorScheduler struct {
	userRepo   domain.UserRepository
	extAccRepo domain.ExternalAccountRepository
	advisorUC  AdvisorAnalyzer
	messenger  Messenger
	loc        *i18n.Localizer
	log        *zap.Logger
}

func NewAdvisorScheduler(
	userRepo domain.UserRepository,
	extAccRepo domain.ExternalAccountRepository,
	advisorUC AdvisorAnalyzer,
	messenger Messenger,
	loc *i18n.Localizer,
	log *zap.Logger,
) *AdvisorScheduler {
	return &AdvisorScheduler{
		userRepo:   userRepo,
		extAccRepo: extAccRepo,
		advisorUC:  advisorUC,
		messenger:  messenger,
		loc:        loc,
		log:        log,
	}
}

// Run starts the hourly check loop.
func (s *AdvisorScheduler) Run(ctx context.Context) {
	s.ProcessPending(ctx)

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.ProcessPending(ctx)
		}
	}
}

// ProcessPending checks for users due for advisor notifications and sends them.
func (s *AdvisorScheduler) ProcessPending(ctx context.Context) {
	now := time.Now()
	users, err := s.userRepo.GetUsersForAdvisor(ctx, now)
	if err != nil {
		s.log.Error("advisor scheduler: failed to get users", zap.Error(err))
		return
	}

	for i := range users {
		s.processUser(ctx, users[i], now)
	}
}

func (s *AdvisorScheduler) processUser(ctx context.Context, u domain.User, now time.Time) {
	advice, err := s.advisorUC.Analyze(ctx, u.ID, now)
	if err != nil {
		s.log.Error("advisor scheduler: analysis failed", zap.Error(err), zap.Int64("user_id", u.ID))
		return
	}

	if advice.Basal == nil && advice.Bolus == nil {
		return
	}

	acc, err := s.extAccRepo.GetByUserID(ctx, u.ID, domain.ProviderTelegram)
	if err != nil || acc == nil {
		s.log.Warn("advisor scheduler: no telegram account", zap.Int64("user_id", u.ID))
		return
	}

	chatID, err := strconv.ParseInt(acc.ExternalID, 10, 64)
	if err != nil {
		s.log.Error("advisor scheduler: invalid external ID", zap.String("external_id", acc.ExternalID))
		return
	}

	text := s.formatAdvisorMessage(advice)
	if err := s.messenger.SendReminder(chatID, text); err != nil {
		s.log.Error("advisor scheduler: failed to send", zap.Error(err))
		return
	}

	if err := s.userRepo.UpdateAdvisorLastSentAt(ctx, u.ID, now); err != nil {
		s.log.Error("advisor scheduler: failed to update last sent", zap.Error(err))
	}
}

func (s *AdvisorScheduler) formatAdvisorMessage(advice *domain.DoseAdvice) string {
	var b strings.Builder
	b.WriteString(s.loc.T("advisor_title"))

	if advice.Basal != nil {
		b.WriteString(s.loc.T("advisor_basal_header"))
		b.WriteString("\n")
		b.WriteString(s.loc.T("advisor_basal_fasting", formatMmol(advice.Basal.FastingAvg), advice.Basal.FastingCount))
		b.WriteString("\n")

		switch advice.Basal.Trend {
		case domain.TrendHigh:
			b.WriteString(s.loc.T("advisor_basal_trend_high"))
		case domain.TrendLow:
			b.WriteString(s.loc.T("advisor_basal_trend_low"))
		case domain.TrendStable:
			b.WriteString(s.loc.T("advisor_basal_trend_stable"))
		}
		b.WriteString("\n")

		if advice.Basal.Trend == domain.TrendStable {
			b.WriteString(s.loc.T("advisor_basal_ok", formatDoseUnits(advice.Basal.CurrentDose)))
		} else {
			b.WriteString(s.loc.T("advisor_basal_suggest",
				formatDoseUnits(advice.Basal.CurrentDose),
				formatDoseUnits(advice.Basal.SuggestedDose)))
		}
	}

	if advice.Bolus != nil {
		b.WriteString("\n")
		b.WriteString(s.loc.T("advisor_bolus_header"))
		b.WriteString("\n")

		if advice.Bolus.PostMealCount > 0 {
			b.WriteString(s.loc.T("advisor_bolus_postmeal", formatMmol(advice.Bolus.PostMealAvg), advice.Bolus.PostMealCount))
			b.WriteString("\n")
		}

		if advice.Bolus.ICRTrend == domain.TrendStable && advice.Bolus.ISFTrend == domain.TrendStable {
			b.WriteString(s.loc.T("advisor_bolus_stable"))
		} else {
			b.WriteString(s.loc.T("advisor_bolus_icr_change",
				formatDoseUnits(advice.Bolus.PreviousICR),
				formatDoseUnits(advice.Bolus.CurrentICR)))
			switch advice.Bolus.ICRTrend {
			case domain.TrendHigh:
				b.WriteString(s.loc.T("advisor_bolus_icr_more"))
			case domain.TrendLow:
				b.WriteString(s.loc.T("advisor_bolus_icr_less"))
			}
			b.WriteString("\n")

			if advice.Bolus.CurrentISF > 0 {
				b.WriteString(s.loc.T("advisor_bolus_isf_change",
					formatMmol(advice.Bolus.PreviousISF),
					formatMmol(advice.Bolus.CurrentISF)))
				switch advice.Bolus.ISFTrend {
				case domain.TrendHigh:
					b.WriteString(s.loc.T("advisor_bolus_isf_more"))
				case domain.TrendLow:
					b.WriteString(s.loc.T("advisor_bolus_isf_less"))
				}
			}
		}
	}

	b.WriteString(s.loc.T("advisor_disclaimer"))

	return b.String()
}

func formatMmol(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}

func formatDoseUnits(dose float64) string {
	return strconv.FormatFloat(dose, 'f', -1, 64)
}
