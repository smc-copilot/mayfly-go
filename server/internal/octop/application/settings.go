package application

import (
	"context"

	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
)

type Settings interface {
	Get(ctx context.Context) (*entity.OctopSettings, error)
	MustGet(ctx context.Context) *entity.OctopSettings
	SaveSettings(ctx context.Context, s *entity.OctopSettings) error
}

type settingsAppImpl struct {
	repo repository.Settings `inject:"T"`
}

var _ Settings = (*settingsAppImpl)(nil)

func (s *settingsAppImpl) Get(ctx context.Context) (*entity.OctopSettings, error) {
	return s.repo.GetOrCreate(ctx)
}

func (s *settingsAppImpl) MustGet(ctx context.Context) *entity.OctopSettings {
	row, err := s.repo.GetOrCreate(ctx)
	if err != nil || row == nil {
		return &entity.OctopSettings{
			EventRetainDays:    entity.DefaultEventRetainDays,
			MetricSampleSec:    entity.DefaultSampleIntervalSec,
			CollectConcurrency: entity.DefaultCollectConcurrency,
			CollectTimeoutSec:  entity.DefaultCollectTimeoutSec,
		}
	}
	return row
}

func (s *settingsAppImpl) SaveSettings(ctx context.Context, in *entity.OctopSettings) error {
	cur, err := s.repo.GetOrCreate(ctx)
	if err != nil {
		return err
	}
	if in.EventRetainDays > 0 {
		cur.EventRetainDays = in.EventRetainDays
	}
	if in.MetricSampleSec > 0 {
		cur.MetricSampleSec = in.MetricSampleSec
	}
	if in.CollectConcurrency > 0 {
		cur.CollectConcurrency = in.CollectConcurrency
	}
	if in.CollectTimeoutSec > 0 {
		cur.CollectTimeoutSec = in.CollectTimeoutSec
	}
	return s.repo.UpdateById(ctx, cur)
}
