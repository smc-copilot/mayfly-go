package repository

import (
	"context"
	"time"

	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/pkg/base"
	"mayfly-go/pkg/model"
)

type Instance interface {
	base.Repo[*entity.OctopInstance]
	ListEnabled(ctx context.Context) ([]*entity.OctopInstance, error)
}

type CasdoorConnection interface {
	base.Repo[*entity.CasdoorConnection]
}

type CasdoorApp interface {
	base.Repo[*entity.CasdoorApplicationSnapshot]
	GetBySource(connectionId uint64, sourceId string) (*entity.CasdoorApplicationSnapshot, error)
	MarkMissingAbsent(ctx context.Context, connectionId uint64, presentSourceIds []string, now time.Time) error
}

type CasdoorUser interface {
	base.Repo[*entity.CasdoorUserSnapshot]
	GetBySource(connectionId uint64, sourceId string) (*entity.CasdoorUserSnapshot, error)
	MarkMissingDisabled(ctx context.Context, connectionId uint64, presentSourceIds []string, now time.Time) error
}

type CasdoorAuth interface {
	base.Repo[*entity.CasdoorAuthRelation]
	GetByKey(connectionId uint64, appSourceId, subjectType, subjectSourceId string) (*entity.CasdoorAuthRelation, error)
	MarkMissingRevoked(ctx context.Context, connectionId uint64, verifiedAppIDs []string, keepKeys []AuthKey, now time.Time) error
}

type AuthKey struct {
	AppSourceId     string
	SubjectType     string
	SubjectSourceId string
}

type IdentityLink interface {
	base.Repo[*entity.OctopIdentityLink]
	GetByOctopUser(instanceId uint64, octopUserId string) (*entity.OctopIdentityLink, error)
	GetByCasdoorUser(connectionId uint64, casdoorSourceId string, instanceId uint64) (*entity.OctopIdentityLink, error)
}

type Event interface {
	base.Repo[*entity.OctopEvent]
	ExistingRecordIDs(ctx context.Context, instanceId uint64, sourceTable string, recordIDs []string) (map[string]struct{}, error)
	PageByQuery(q *entity.EventQuery, allowedInstanceIds []uint64) (*model.PageResult[*entity.OctopEvent], error)
	Summarize(q *entity.EventQuery, allowedInstanceIds []uint64, groupBy string) ([]*EventSummary, error)
	ListForExport(q *entity.EventQuery, allowedInstanceIds []uint64, limit int) ([]*entity.OctopEvent, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}

type EventSummary struct {
	GroupKey     string     `json:"groupKey"`
	DisplayName  string     `json:"displayName"`
	Total        int64      `json:"total"`
	FailCount    int64      `json:"failCount"`
	InputTokens  int64      `json:"inputTokens"`
	OutputTokens int64      `json:"outputTokens"`
	LastActivity *time.Time `json:"lastActivity"`
}

type Checkpoint interface {
	base.Repo[*entity.CollectorCheckpoint]
	GetByTable(instanceId uint64, sourceTable string) (*entity.CollectorCheckpoint, error)
}

type CollectorRun interface {
	base.Repo[*entity.CollectorRun]
	RecentByInstance(instanceId uint64, kind string, limit int) ([]*entity.CollectorRun, error)
}

type MetricSample interface {
	base.Repo[*entity.OctopMetricSample]
	GetRange(instanceId uint64, start, end time.Time) ([]*entity.OctopMetricSample, error)
	Latest(instanceId uint64) (*entity.OctopMetricSample, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}

type Settings interface {
	base.Repo[*entity.OctopSettings]
	GetOrCreate(ctx context.Context) (*entity.OctopSettings, error)
}

type ReadConnection interface {
	base.Repo[*entity.OctopReadConnection]
	GetByInstanceType(ctx context.Context, instanceId uint64, connType string) (*entity.OctopReadConnection, error)
	ListByInstance(ctx context.Context, instanceId uint64) ([]*entity.OctopReadConnection, error)
}
