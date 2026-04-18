package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/gmtantsevov/shuggabuddy/internal/domain"
	"github.com/gmtantsevov/shuggabuddy/internal/domain/mocks"
)

type mockSyncer struct {
	calls []int64
	err   error
}

func (m *mockSyncer) SyncUser(_ context.Context, conn *domain.CGMConnection) error {
	m.calls = append(m.calls, conn.UserID)
	return m.err
}

func TestCGMSyncProcessAllIteratesConnections(t *testing.T) {
	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)

	conns := []domain.CGMConnection{
		{ID: 1, UserID: 10, Active: true},
		{ID: 2, UserID: 20, Active: true},
		{ID: 3, UserID: 30, Active: true},
	}
	cgmRepo.EXPECT().GetAllActive(gomock.Any()).Return(conns, nil)

	syncer := &mockSyncer{}
	s := NewCGMSyncScheduler(cgmRepo, syncer, zaptest.NewLogger(t))

	s.ProcessAll(context.Background())

	assert.Equal(t, []int64{10, 20, 30}, syncer.calls)
}

func TestCGMSyncProcessAllContinuesOnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)

	conns := []domain.CGMConnection{
		{ID: 1, UserID: 10, Active: true},
		{ID: 2, UserID: 20, Active: true},
	}
	cgmRepo.EXPECT().GetAllActive(gomock.Any()).Return(conns, nil)

	syncer := &mockSyncer{err: errors.New("network timeout")}
	s := NewCGMSyncScheduler(cgmRepo, syncer, zaptest.NewLogger(t))

	s.ProcessAll(context.Background())

	assert.Len(t, syncer.calls, 2, "should attempt all users even with errors")
}

func TestCGMSyncProcessAllHandlesRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	cgmRepo := mocks.NewMockCGMConnectionRepository(ctrl)

	cgmRepo.EXPECT().GetAllActive(gomock.Any()).Return(nil, errors.New("db down"))

	syncer := &mockSyncer{}
	s := NewCGMSyncScheduler(cgmRepo, syncer, zaptest.NewLogger(t))

	s.ProcessAll(context.Background())

	assert.Empty(t, syncer.calls)
}
