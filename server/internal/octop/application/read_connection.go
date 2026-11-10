package application

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mayfly-go/internal/octop/application/dto"
	"mayfly-go/internal/octop/domain/business"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/internal/octop/domain/repository"
	"mayfly-go/internal/octop/imsg"
	"mayfly-go/internal/octop/infra/octopapi"
	"mayfly-go/internal/pkg/utils"
	"mayfly-go/pkg/base"
	"mayfly-go/pkg/errorx"
	"mayfly-go/pkg/logx"
	"mayfly-go/pkg/model"
)

const previewTimeout = 15 * time.Second

// previewFactory 构造 Octop 预览客户端，测试可替换。
type previewFactory func(cfg octopapi.PreviewConfig) business.PreviewClient

func defaultPreviewFactory(cfg octopapi.PreviewConfig) business.PreviewClient {
	return octopapi.NewPreviewClient(cfg)
}

// ReadConnectionApp 实例业务只读连接配置（Octop 预览 API / Langfuse）。
type ReadConnectionApp interface {
	// Get 返回脱敏配置；未配置时返回 not_configured 的空视图（Revision=0）。
	Get(ctx context.Context, instanceId uint64, connType string) (*dto.ReadConnectionView, error)
	// List 返回该实例全部类型的脱敏配置，固定顺序 octop_api、langfuse。
	List(ctx context.Context, instanceId uint64) ([]*dto.ReadConnectionView, error)
	// Save 保存配置：空 Secret 保留旧值，ClearSecret 清除，Revision 乐观并发校验。
	Save(ctx context.Context, instanceId uint64, req *dto.ReadConnectionSave) (*dto.ReadConnectionView, error)
	// Test 只读探测候选配置，不保存配置。
	Test(ctx context.Context, instanceId uint64, req *dto.ReadConnectionTest) (*dto.ReadConnectionTestResult, error)
}

// instanceAccessor 业务读取侧只需实例可见性校验，便于单测注入。
type instanceAccessor interface {
	GetVisible(ctx context.Context, id uint64) (*entity.OctopInstance, error)
}

type readConnectionAppImpl struct {
	connRepo    repository.ReadConnection `inject:"T"`
	instanceApp instanceAccessor          `inject:"T"`
	newPreview  previewFactory
}

var _ ReadConnectionApp = (*readConnectionAppImpl)(nil)

var readConnTypes = []string{entity.ReadConnTypeOctopAPI, entity.ReadConnTypeLangfuse}

func validReadConnType(t string) bool {
	for _, v := range readConnTypes {
		if v == t {
			return true
		}
	}
	return false
}

func (a *readConnectionAppImpl) previewFactory() previewFactory {
	if a.newPreview != nil {
		return a.newPreview
	}
	return defaultPreviewFactory
}

func (a *readConnectionAppImpl) Get(ctx context.Context, instanceId uint64, connType string) (*dto.ReadConnectionView, error) {
	if !validReadConnType(connType) {
		return nil, errorx.NewBizI(ctx, imsg.ErrReadConnNotFound)
	}
	if _, err := a.instanceApp.GetVisible(ctx, instanceId); err != nil {
		return nil, err
	}
	row, err := a.connRepo.GetByInstanceType(ctx, instanceId, connType)
	if err != nil {
		return nil, err
	}
	return toReadConnectionView(instanceId, connType, row), nil
}

func (a *readConnectionAppImpl) List(ctx context.Context, instanceId uint64) ([]*dto.ReadConnectionView, error) {
	if _, err := a.instanceApp.GetVisible(ctx, instanceId); err != nil {
		return nil, err
	}
	rows, err := a.connRepo.ListByInstance(ctx, instanceId)
	if err != nil {
		return nil, err
	}
	byType := make(map[string]*entity.OctopReadConnection, len(rows))
	for _, r := range rows {
		byType[r.Type] = r
	}
	out := make([]*dto.ReadConnectionView, 0, len(readConnTypes))
	for _, t := range readConnTypes {
		out = append(out, toReadConnectionView(instanceId, t, byType[t]))
	}
	return out, nil
}

func (a *readConnectionAppImpl) Save(ctx context.Context, instanceId uint64, req *dto.ReadConnectionSave) (*dto.ReadConnectionView, error) {
	if req == nil || !validReadConnType(req.Type) {
		return nil, errorx.NewBizI(ctx, imsg.ErrReadConnNotFound)
	}
	if _, err := a.instanceApp.GetVisible(ctx, instanceId); err != nil {
		return nil, err
	}
	baseURL := strings.TrimSpace(req.BaseURL)
	if err := validateReadBaseURL(baseURL); err != nil {
		return nil, errorx.NewBiz(err.Error())
	}
	projectId := strings.TrimSpace(req.ProjectId)
	publicKey := strings.TrimSpace(req.PublicKey)

	old, err := a.connRepo.GetByInstanceType(ctx, instanceId, req.Type)
	if err != nil {
		return nil, err
	}
	if old == nil {
		return a.create(ctx, instanceId, req, baseURL, projectId, publicKey)
	}
	if req.Revision != old.Revision {
		return nil, errorx.NewBizI(ctx, imsg.ErrReadConnRevisionConflict)
	}

	secretEnc, configured, rotatedAt, secretChanged, err := resolveSecret(old, req.Secret, req.ClearSecret)
	if err != nil {
		return nil, err
	}
	changed := secretChanged || baseURL != old.BaseURL || projectId != old.ProjectId || publicKey != old.PublicKey

	next := &entity.OctopReadConnection{
		Type: req.Type, BaseURL: baseURL, ProjectId: projectId, PublicKey: publicKey,
		SecretEnc: secretEnc, CredentialConfigured: configured,
	}
	patch := base.NewPatch().
		Set("base_url", baseURL).
		Set("project_id", projectId).
		Set("public_key", publicKey).
		Set("secret_enc", secretEnc).
		Set("credential_configured", configured).
		Set("credential_rotated_at", rotatedAt).
		Set("revision", old.Revision+1)
	if changed {
		// 配置或凭据变化后旧验证结果作废，避免“旧版本验证通过”覆盖新凭据版本。
		patch = patch.
			Set("verify_state", initialVerifyState(next)).
			Set("verified_at", nil).
			Set("verify_error", "")
	}
	n, err := a.connRepo.UpdateColsByCond(ctx, patch, model.NewCond().Eq("id", old.Id).Eq("revision", old.Revision))
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errorx.NewBizI(ctx, imsg.ErrReadConnRevisionConflict)
	}
	row, err := a.connRepo.GetByInstanceType(ctx, instanceId, req.Type)
	if err != nil {
		return nil, err
	}
	return toReadConnectionView(instanceId, req.Type, row), nil
}

func (a *readConnectionAppImpl) create(ctx context.Context, instanceId uint64, req *dto.ReadConnectionSave, baseURL, projectId, publicKey string) (*dto.ReadConnectionView, error) {
	if req.Revision != 0 {
		return nil, errorx.NewBizI(ctx, imsg.ErrReadConnRevisionConflict)
	}
	row := &entity.OctopReadConnection{
		InstanceId: instanceId, Type: req.Type,
		BaseURL: baseURL, ProjectId: projectId, PublicKey: publicKey,
		CredentialConfigured: -1, Revision: 1,
	}
	if req.Secret != "" {
		enc, err := utils.PwdAesEncrypt(req.Secret)
		if err != nil {
			return nil, err
		}
		now := time.Now()
		row.SecretEnc = enc
		row.CredentialConfigured = 1
		row.CredentialRotatedAt = &now
	}
	row.VerifyState = initialVerifyState(row)
	if err := a.connRepo.Insert(ctx, row); err != nil {
		// 并发首次保存：唯一键冲突说明他人已先创建，按版本冲突处理。
		if exist, gerr := a.connRepo.GetByInstanceType(ctx, instanceId, req.Type); gerr == nil && exist != nil {
			return nil, errorx.NewBizI(ctx, imsg.ErrReadConnRevisionConflict)
		}
		return nil, err
	}
	return toReadConnectionView(instanceId, req.Type, row), nil
}

func (a *readConnectionAppImpl) Test(ctx context.Context, instanceId uint64, req *dto.ReadConnectionTest) (*dto.ReadConnectionTestResult, error) {
	if req == nil || !validReadConnType(req.Type) {
		return nil, errorx.NewBizI(ctx, imsg.ErrReadConnNotFound)
	}
	if _, err := a.instanceApp.GetVisible(ctx, instanceId); err != nil {
		return nil, err
	}
	stored, err := a.connRepo.GetByInstanceType(ctx, instanceId, req.Type)
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimSpace(req.BaseURL)
	projectId := strings.TrimSpace(req.ProjectId)
	publicKey := strings.TrimSpace(req.PublicKey)
	now := time.Now()
	res := &dto.ReadConnectionTestResult{At: &now}

	if baseURL == "" {
		res.State = entity.VerifyStateNotConfigured
		res.Message = "未填写服务地址"
		return res, nil
	}
	if err := validateReadBaseURL(baseURL); err != nil {
		res.State = entity.VerifyStateFailed
		res.Message = err.Error()
		return res, nil
	}

	// 已保存凭据仅在目标地址未变化时复用，防止借测试把已保存凭据发往任意主机。
	sameTarget := stored != nil && sameBaseURL(stored.BaseURL, baseURL)
	secret := req.Secret
	authHeader := ""
	if secret == "" && sameTarget {
		secret, err = utils.PwdAesDecrypt(stored.SecretEnc)
		if err != nil {
			return nil, err
		}
		if req.Type == entity.ReadConnTypeOctopAPI && stored.AuthHeaderEnc != "" {
			authHeader, err = utils.PwdAesDecrypt(stored.AuthHeaderEnc)
			if err != nil {
				return nil, err
			}
		}
	}

	switch req.Type {
	case entity.ReadConnTypeOctopAPI:
		a.probePreview(ctx, res, octopapi.PreviewConfig{
			BaseURL: baseURL, Secret: secret, AuthHeader: authHeader, Timeout: previewTimeout,
		})
	default: // langfuse
		if publicKey == "" || secret == "" {
			res.State = entity.VerifyStateFailed
			res.Message = "Langfuse 需要公钥和密钥"
		} else {
			// S4 之前不实际调用 Langfuse：只校验配置形态，不声称连通。
			res.State = entity.VerifyStateNotVerified
			res.Message = "配置格式有效，连通性待追踪读取能力上线后验证"
		}
	}
	res.Ok = res.State == entity.VerifyStateReady

	a.recordVerify(ctx, stored, req, baseURL, projectId, publicKey, res)
	return res, nil
}

func (a *readConnectionAppImpl) probePreview(ctx context.Context, res *dto.ReadConnectionTestResult, cfg octopapi.PreviewConfig) {
	pctx, cancel := context.WithTimeout(ctx, previewTimeout)
	defer cancel()
	err := a.previewFactory()(cfg).Probe(pctx)
	res.State, res.Message = classifyPreviewErr(err)
}

// recordVerify 仅当测试针对已保存且未变更的配置版本时，把结果记到该版本；候选配置的测试不落库。
func (a *readConnectionAppImpl) recordVerify(ctx context.Context, stored *entity.OctopReadConnection, req *dto.ReadConnectionTest, baseURL, projectId, publicKey string, res *dto.ReadConnectionTestResult) {
	if stored == nil || req.Revision <= 0 || req.Revision != stored.Revision || req.Secret != "" {
		return
	}
	if !sameBaseURL(stored.BaseURL, baseURL) || stored.ProjectId != projectId || stored.PublicKey != publicKey {
		return
	}
	patch := base.NewPatch().
		Set("verify_state", res.State).
		Set("verified_at", res.At).
		Set("verify_error", truncateRunes(verifyErrorOf(res), 250))
	if _, err := a.connRepo.UpdateColsByCond(ctx, patch, model.NewCond().Eq("id", stored.Id).Eq("revision", stored.Revision)); err != nil {
		logx.Errorf("octop record read connection verify instance=%d type=%s err=%v", stored.InstanceId, stored.Type, err)
	}
}

func verifyErrorOf(res *dto.ReadConnectionTestResult) string {
	if res.State == entity.VerifyStateReady {
		return ""
	}
	return res.Message
}

func classifyPreviewErr(err error) (string, string) {
	if err == nil {
		return entity.VerifyStateReady, "连接正常"
	}
	var he *octopapi.PreviewHTTPError
	switch {
	case errors.As(err, &he) && (he.StatusCode == http.StatusUnauthorized || he.StatusCode == http.StatusForbidden):
		return entity.VerifyStateForbidden, "凭据被拒绝 (http " + strconv.Itoa(he.StatusCode) + ")"
	case errors.As(err, &he):
		return entity.VerifyStateFailed, "服务返回异常 (http " + strconv.Itoa(he.StatusCode) + ")"
	case errors.Is(err, octopapi.ErrPreviewConfig):
		return entity.VerifyStateFailed, trimErr(err)
	case errors.Is(err, context.DeadlineExceeded):
		return entity.VerifyStateFailed, "请求超时"
	default:
		return entity.VerifyStateFailed, trimErr(err)
	}
}

// resolveSecret 计算保存后的密钥密文、配置标志与轮换时间。
func resolveSecret(old *entity.OctopReadConnection, secret string, clear bool) (enc string, configured int8, rotatedAt *time.Time, changed bool, err error) {
	switch {
	case secret != "":
		enc, err = utils.PwdAesEncrypt(secret)
		if err != nil {
			return "", 0, nil, false, err
		}
		now := time.Now()
		return enc, 1, &now, true, nil
	case clear:
		now := time.Now()
		return "", -1, &now, old.SecretEnc != "", nil
	default:
		return old.SecretEnc, old.CredentialConfigured, old.CredentialRotatedAt, false, nil
	}
}

// initialVerifyState 配置写入后的初始验证状态：未配置地址为 not_configured，其余为 not_verified。
func initialVerifyState(c *entity.OctopReadConnection) string {
	if c.BaseURL == "" {
		return entity.VerifyStateNotConfigured
	}
	if c.Type == entity.ReadConnTypeLangfuse && (c.PublicKey == "" || !c.HasCredential()) {
		return entity.VerifyStateNotConfigured
	}
	return entity.VerifyStateNotVerified
}

func toReadConnectionView(instanceId uint64, connType string, row *entity.OctopReadConnection) *dto.ReadConnectionView {
	if row == nil {
		return &dto.ReadConnectionView{
			InstanceId: instanceId, Type: connType, VerifyState: entity.VerifyStateNotConfigured,
		}
	}
	return &dto.ReadConnectionView{
		Id: row.Id, InstanceId: row.InstanceId, Type: row.Type,
		BaseURL: row.BaseURL, ProjectId: row.ProjectId, PublicKey: row.PublicKey,
		CredentialConfigured: row.HasCredential(), CredentialRotatedAt: row.CredentialRotatedAt,
		Revision: row.Revision, VerifyState: row.VerifyState, VerifiedAt: row.VerifiedAt, VerifyError: row.VerifyError,
	}
}

// validateReadBaseURL 空地址合法（表示未配置）；否则必须是不带 userinfo/query/fragment 的绝对 http(s) 地址。
func validateReadBaseURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("服务地址必须是绝对的 http(s) 地址")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("服务地址不能包含用户信息、查询参数或片段")
	}
	return nil
}

func sameBaseURL(a, b string) bool {
	return strings.EqualFold(strings.TrimRight(strings.TrimSpace(a), "/"), strings.TrimRight(strings.TrimSpace(b), "/"))
}

// loadPreviewClient 读取并解密 octop_api 配置，构造预览客户端。
// 未配置（无记录、无地址或无任何凭据）时返回 (nil, VerifyStateNotConfigured, nil)。
func loadPreviewClient(ctx context.Context, repo repository.ReadConnection, factory previewFactory, instanceId uint64) (business.PreviewClient, *entity.OctopReadConnection, error) {
	row, err := repo.GetByInstanceType(ctx, instanceId, entity.ReadConnTypeOctopAPI)
	if err != nil {
		return nil, nil, err
	}
	if row == nil || row.BaseURL == "" || (row.SecretEnc == "" && row.AuthHeaderEnc == "") {
		return nil, row, nil
	}
	secret, err := utils.PwdAesDecrypt(row.SecretEnc)
	if err != nil {
		return nil, row, err
	}
	authHeader, err := utils.PwdAesDecrypt(row.AuthHeaderEnc)
	if err != nil {
		return nil, row, err
	}
	if factory == nil {
		factory = defaultPreviewFactory
	}
	return factory(octopapi.PreviewConfig{
		BaseURL: row.BaseURL, Secret: secret, AuthHeader: authHeader, Timeout: previewTimeout,
	}), row, nil
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
