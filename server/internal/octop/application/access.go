package application

import (
	"context"

	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/internal/pkg/consts"
	tagentity "mayfly-go/internal/tag/domain/entity"
	"mayfly-go/pkg/contextx"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/model"
)

func (a *instanceAppImpl) VisibleInstanceIDs(ctx context.Context) ([]uint64, error) {
	list, err := a.ListVisible(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(list))
	for _, inst := range list {
		ids = append(ids, inst.Id)
	}
	return ids, nil
}

func (a *instanceAppImpl) ListVisible(ctx context.Context) ([]*entity.OctopInstance, error) {
	all, err := a.ListByCond(model.NewCond())
	if err != nil {
		return nil, err
	}
	out := make([]*entity.OctopInstance, 0, len(all))
	for _, inst := range all {
		if err := a.AssertInstanceAccess(ctx, inst); err != nil {
			continue
		}
		out = append(out, inst)
	}
	return out, nil
}

func (a *instanceAppImpl) AssertInstanceAccess(ctx context.Context, inst *entity.OctopInstance) error {
	if inst == nil {
		return errorx.NewBizI(ctx, imsg.ErrInstanceNotFound)
	}
	la := contextx.GetLoginAccount(ctx)
	if la == nil || la.Id == consts.AdminId {
		return nil
	}
	if inst.MachineId == 0 {
		return errorx.NewBizI(ctx, imsg.ErrInstanceNoPermission)
	}
	machine, err := a.machineApp.GetById(inst.MachineId)
	if err != nil {
		if model.IsNotFound(err) {
			return errorx.NewBizI(ctx, imsg.ErrInstanceNoPermission)
		}
		return err
	}
	if err := a.tagApp.CanAccessByCode(ctx, int8(tagentity.TagTypeMachine), machine.Code); err != nil {
		return errorx.NewBizI(ctx, imsg.ErrInstanceNoPermission)
	}
	return nil
}

func (a *instanceAppImpl) mustGetVisible(ctx context.Context, id uint64) (*entity.OctopInstance, error) {
	inst, err := a.GetById(id)
	if err != nil {
		if model.IsNotFound(err) {
			return nil, errorx.NewBizI(ctx, imsg.ErrInstanceNotFound)
		}
		return nil, err
	}
	if err := a.AssertInstanceAccess(ctx, inst); err != nil {
		return nil, err
	}
	return inst, nil
}
