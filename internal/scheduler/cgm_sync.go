package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
)

// CGMSyncer syncs CGM data for a single connection.
type CGMSyncer interface {
	SyncUser(ctx context.Context, conn *domain.CGMConnection) error
}

// CGMSyncScheduler periodically syncs CGM data for all active connections.
type CGMSyncScheduler struct {
	cgmRepo domain.CGMConnectionRepository
	syncer  CGMSyncer
	log     *zap.Logger
}

func NewCGMSyncScheduler(
	cgmRepo domain.CGMConnectionRepository,
	syncer CGMSyncer,
	log *zap.Logger,
) *CGMSyncScheduler {
	return &CGMSyncScheduler{
		cgmRepo: cgmRepo,
		syncer:  syncer,
		log:     log,
	}
}

// Run starts the periodic sync loop.
func (s *CGMSyncScheduler) Run(ctx context.Context) {
	s.ProcessAll(ctx)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.ProcessAll(ctx)
		}
	}
}

// ProcessAll syncs CGM data for all active connections.
func (s *CGMSyncScheduler) ProcessAll(ctx context.Context) {
	connections, err := s.cgmRepo.GetAllActive(ctx)
	if err != nil {
		s.log.Error("cgm sync: failed to get connections", zap.Error(err))
		return
	}

	for i := range connections {
		if err := s.syncer.SyncUser(ctx, &connections[i]); err != nil {
			s.log.Error("cgm sync: failed to sync user",
				zap.Error(err),
				zap.Int64("user_id", connections[i].UserID),
			)
		}
	}
}
