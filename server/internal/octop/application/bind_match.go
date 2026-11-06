package application

import (
	"context"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/model"
)

func (a *instanceAppImpl) ListBindMatches(ctx context.Context, connectionId uint64) ([]*dto.AppBindMatch, error) {
	if connectionId == 0 {
		return []*dto.AppBindMatch{}, nil
	}
	apps, err := a.appRepo.SelectByCond(model.NewCond().Eq("connection_id", connectionId))
	if err != nil {
		return nil, err
	}
	visible, err := a.ListVisible(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*dto.AppBindMatch, 0, len(apps))
	for _, app := range apps {
		redirects := DecodeRedirectURIs(app.RedirectURIs)
		item := &dto.AppBindMatch{
			AppSnapshotId: app.Id,
			SourceId:      app.SourceId,
			Name:          app.Name,
			DisplayName:   app.DisplayName,
			RedirectURIs:  redirects,
			Existing:      make([]dto.ExistingBind, 0),
			Candidates:    make([]dto.MatchCandidate, 0),
		}
		for _, inst := range visible {
			if inst.CasdoorAppSnapshotId == app.Id {
				item.Existing = append(item.Existing, dto.ExistingBind{
					InstanceId: inst.Id, InstanceName: inst.Name, InstanceCode: inst.Code,
				})
			}
			ok, hit := matchInstanceToRedirects(inst, redirects)
			if !ok {
				continue
			}
			item.Candidates = append(item.Candidates, dto.MatchCandidate{
				InstanceId:           inst.Id,
				InstanceName:         inst.Name,
				InstanceCode:         inst.Code,
				CallbackURL:          inst.CallbackURL,
				ApiURL:               inst.ApiURL,
				MatchedRedirectURI:   hit.RedirectRaw,
				MatchBasis:           hit.Basis,
				NormalizedCallback:   hit.NormalizedCallback,
				NormalizedRedirect:   hit.NormalizedRedirect,
				CurrentAppSnapshotId: inst.CasdoorAppSnapshotId,
			})
		}
		item.MatchStatus = classifyBindMatch(redirects, len(item.Candidates))
		out = append(out, item)
	}
	return out, nil
}

func (a *instanceAppImpl) BindCasdoorApp(ctx context.Context, instanceId, appSnapshotId uint64, confirmOverwrite bool) error {
	inst, err := a.mustGetVisible(ctx, instanceId)
	if err != nil {
		return err
	}
	if appSnapshotId == 0 {
		inst.CasdoorAppSnapshotId = 0
		return a.UpdateById(ctx, inst)
	}
	app, err := a.appRepo.GetById(appSnapshotId)
	if err != nil {
		if model.IsNotFound(err) {
			return errorx.NewBizI(ctx, imsg.ErrBindAppNotFound)
		}
		return err
	}
	if inst.CasdoorAppSnapshotId != 0 && inst.CasdoorAppSnapshotId != app.Id && !confirmOverwrite {
		return errorx.NewBizI(ctx, imsg.ErrBindOverwriteRequired)
	}
	inst.CasdoorAppSnapshotId = app.Id
	inst.CasdoorConnectionId = app.ConnectionId
	return a.UpdateById(ctx, inst)
}
