package business

import (
	"context"

	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/pkg/model"
)

// Reader PostgreSQL 业务只读契约。上层只依赖本接口，不直接依赖 PG 适配细节。
type Reader interface {
	PageUsers(ctx context.Context, q *entity.BusinessQuery) (*model.PageResult[*entity.BizUser], *entity.SourceMeta, error)
	PageUserOptions(ctx context.Context, q *entity.BusinessQuery) (*model.PageResult[*entity.BizUserOption], *entity.SourceMeta, error)
	GetUser(ctx context.Context, userId string) (*entity.BizUser, *entity.SourceMeta, error)
	CountExperts(ctx context.Context, userId string) (entity.StatValue, error)
	CountThreads(ctx context.Context, userId string) (entity.StatValue, error)
	CountTemplates(ctx context.Context, userId string) (entity.StatValue, error)
	CountSkillPackages(ctx context.Context, userId string) (entity.StatValue, error)
	PageExperts(ctx context.Context, userId string, q *entity.BusinessQuery) (*model.PageResult[*entity.BizExpert], *entity.SourceMeta, error)
	GetExpert(ctx context.Context, userId, expertId string) (*entity.BizExpert, *entity.SourceMeta, error)
	PageThreads(ctx context.Context, userId string, q *entity.BusinessQuery) (*model.PageResult[*entity.BizThread], *entity.SourceMeta, error)
	GetThread(ctx context.Context, userId, threadId string) (*entity.BizThread, *entity.SourceMeta, error)
	PageExpertTemplates(ctx context.Context, userId string, q *entity.BusinessQuery) (*model.PageResult[*entity.BizExpertTemplate], *entity.SourceMeta, error)
	GetExpertTemplate(ctx context.Context, userId, templateId string) (*entity.BizExpertTemplate, *entity.SourceMeta, error)
	PageSkillPackages(ctx context.Context, userId string, q *entity.BusinessQuery) (*model.PageResult[*entity.BizSkillPackage], *entity.SourceMeta, error)
	GetSkillPackage(ctx context.Context, userId, packageId string) (*entity.BizSkillPackage, *entity.SourceMeta, error)
	PageSkills(ctx context.Context, userId, packageId string, q *entity.BusinessQuery) (*model.PageResult[*entity.BizSkill], *entity.SourceMeta, error)
	GetSkill(ctx context.Context, userId, packageId, slug string) (*entity.BizSkill, *entity.SourceMeta, error)
}

// PreviewClient Octop 预览 API 契约（技能条目、文件目录与内容）。
type PreviewClient interface {
	ListSkillFiles(ctx context.Context, packageId, slug string) ([]entity.BizFileEntry, error)
	ListTemplateFiles(ctx context.Context, templateId string) ([]entity.BizFileEntry, error)
	GetFileContent(ctx context.Context, fileId string) (*entity.BizFileContent, error)
	Probe(ctx context.Context) error
}
