// Package businessread 实现 Octop PostgreSQL 业务只读读取器（business.Reader）。
//
// 约定：
//   - 只读：仅 SELECT；显式列清单，禁止 SELECT *；值一律走 $n 占位符；表名经 quoteIdent。
//   - 返回 (结果, SourceMeta, error)：查询失败时 error 非空，同时 SourceMeta/StatValue 携带
//     分类状态（unsupported / forbidden / failed），调用方不得把失败当成 0。
//   - 用户 ID：Octop users.id 为整数，API 层为字符串；仅接受十进制非负整数字符串，
//     其它表（agents/threads）用 user_id::text 比较，避免 int/text 列类型差异。
//
// 业务字段对齐 Octop 源码及 2026-11-10 核验的 PostgreSQL schema：
//
//	users(id, username, display_name, disabled, role, created_at) -- disabled 为整数 0/1
//	agents(agent_id, user_id, kind, name, description, default_model, enabled, last_state, config_json, template_name, published_expert_id, skill_package_ids, knowledge_base_ids, mcp_servers, created_at, updated_at)
//	threads(thread_id, user_id, agent_id, title, channel_type, created_at, last_active) -- 时间为 epoch 秒，last_active=0 表示无消息
//	published_experts(published_expert_id, created_by, name, description, source_agent_id, slug, created_at, updated_at)
//	skill_packages(skill_package_id, created_by, name, description, skill_count, shared, updated_at)
//	skills(skill_package_id, slug, name, description, source)
package businessread

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"mayfly-go/internal/db/dbm/dbi"
	"mayfly-go/internal/octop/domain/business"
	"mayfly-go/internal/octop/domain/entity"
	"mayfly-go/pkg/model"
)

var _ business.Reader = (*Reader)(nil)

const (
	// SourceName SourceMeta.Source 取值。
	SourceName = "octop_postgres"

	defaultPageSize = 10
	maxPageSize     = 100

	kindExpert = "expert"
)

var (
	// ErrNotFound 目标记录不存在（或不属于该用户）。
	ErrNotFound = errors.New("octop business record not found")
	// ErrInvalidUserId 用户 ID 不是十进制非负整数字符串。
	ErrInvalidUserId = errors.New("invalid octop user id")
	// ErrInvalidQuery 查询参数非法（时间字段/时间格式等）。
	ErrInvalidQuery = errors.New("invalid octop business query")
)

// Queryer 只读查询抽象，与 pgcollect.Queryer 同形。
type Queryer interface {
	QueryContext(ctx context.Context, querySQL string, args ...any) ([]*dbi.QueryColumn, []map[string]any, error)
}

// Tables 业务表名（可由部署差异覆盖）。
type Tables struct {
	Users            string
	Agents           string
	Threads          string
	PublishedExperts string
	SkillPackages    string
	Skills           string
}

// DefaultTables 对齐公开 Octop schema。
func DefaultTables() Tables {
	return Tables{
		Users:            "users",
		Agents:           "agents",
		Threads:          "threads",
		PublishedExperts: "published_experts",
		SkillPackages:    "skill_packages",
		Skills:           "skills",
	}
}

// Option 构造选项。
type Option func(*Reader)

// WithTables 覆盖表名，空字段保持默认。
func WithTables(t Tables) Option {
	return func(r *Reader) {
		d := r.tables
		pick := func(v, def string) string {
			if strings.TrimSpace(v) == "" {
				return def
			}
			return v
		}
		r.tables = Tables{
			Users:            pick(t.Users, d.Users),
			Agents:           pick(t.Agents, d.Agents),
			Threads:          pick(t.Threads, d.Threads),
			PublishedExperts: pick(t.PublishedExperts, d.PublishedExperts),
			SkillPackages:    pick(t.SkillPackages, d.SkillPackages),
			Skills:           pick(t.Skills, d.Skills),
		}
	}
}

// WithClock 注入时钟（测试用）。
func WithClock(now func() time.Time) Option {
	return func(r *Reader) {
		if now != nil {
			r.now = now
		}
	}
}

// Reader PostgreSQL 业务只读读取器。
type Reader struct {
	q      Queryer
	tables Tables
	now    func() time.Time
}

// NewReader 创建读取器。
func NewReader(q Queryer, opts ...Option) *Reader {
	r := &Reader{q: q, tables: DefaultTables(), now: time.Now}
	for _, o := range opts {
		o(r)
	}
	return r
}

// ---------------------------------------------------------------- users

const userSelectCols = `u.id AS user_id, u.username AS username, u.display_name AS display_name, ` +
	`(u.disabled = 0) AS enabled, u.role AS role, u.created_at AS created_at, ` +
	`COALESCE(e.cnt, 0) AS expert_count, COALESCE(t.cnt, 0) AS thread_count, t.last_active AS last_active`

// usersFrom 专家数/会话数来自两个独立分组子查询，再按用户 id 连接，
// 禁止 agents×threads 直接 JOIN（会使计数相乘）。
func (r *Reader) usersFrom(a *args) string {
	return fmt.Sprintf(
		"FROM %s u "+
			"LEFT JOIN (SELECT user_id::text AS uid, COUNT(DISTINCT agent_id) AS cnt FROM %s WHERE kind = %s GROUP BY user_id::text) e ON e.uid = u.id::text "+
			"LEFT JOIN (SELECT user_id::text AS uid, COUNT(DISTINCT thread_id) AS cnt, MAX(last_active) AS last_active FROM %s GROUP BY user_id::text) t ON t.uid = u.id::text",
		quoteIdent(r.tables.Users),
		quoteIdent(r.tables.Agents), a.add(kindExpert),
		quoteIdent(r.tables.Threads),
	)
}

func (r *Reader) userFilters(a *args, q *entity.BusinessQuery) []string {
	var conds []string
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		p := a.add(likePattern(kw))
		conds = append(conds, fmt.Sprintf(`(u.username ILIKE %s ESCAPE '\' OR u.display_name ILIKE %s ESCAPE '\')`, p, p))
	}
	if q.Enabled != nil {
		conds = append(conds, "u.disabled = "+a.add(1-enabledFlag(*q.Enabled)))
	}
	return conds
}

// PageUsers 分页用户（含专家数/会话数）。
func (r *Reader) PageUsers(ctx context.Context, q *entity.BusinessQuery) (*model.PageResult[*entity.BizUser], *entity.SourceMeta, error) {
	q = normalizeQuery(q)
	a := &args{}
	from := r.usersFrom(a)
	conds := r.userFilters(a, q)
	where := whereClause(conds)
	// 计数只需 users + 过滤（过滤仅引用 u 列）；单独构造参数避免带上子查询参数。
	ca := &args{}
	cconds := r.userFilters(ca, q)
	countSQL := fmt.Sprintf("SELECT COUNT(*) AS total FROM %s u%s", quoteIdent(r.tables.Users), whereClause(cconds))
	listSQL := fmt.Sprintf("SELECT %s %s%s ORDER BY u.id ASC", userSelectCols, from, where)
	return runPage(ctx, r, countSQL, ca, listSQL, a, q, r.mapUser)
}

// PageUserOptions 用户选择器。
func (r *Reader) PageUserOptions(ctx context.Context, q *entity.BusinessQuery) (*model.PageResult[*entity.BizUserOption], *entity.SourceMeta, error) {
	q = normalizeQuery(q)
	a := &args{}
	conds := r.userFilters(a, q)
	where := whereClause(conds)
	countSQL := fmt.Sprintf("SELECT COUNT(*) AS total FROM %s u%s", quoteIdent(r.tables.Users), where)
	listSQL := fmt.Sprintf("SELECT u.id AS user_id, u.username AS username, u.display_name AS display_name FROM %s u%s ORDER BY u.id ASC",
		quoteIdent(r.tables.Users), where)
	ca := &args{vals: append([]any(nil), a.vals...)}
	return runPage(ctx, r, countSQL, ca, listSQL, a, q, func(row map[string]any) *entity.BizUserOption {
		return &entity.BizUserOption{
			UserId:      stringify(row["user_id"]),
			Username:    stringify(row["username"]),
			DisplayName: stringify(row["display_name"]),
		}
	})
}

// GetUser 单个用户。
func (r *Reader) GetUser(ctx context.Context, userId string) (*entity.BizUser, *entity.SourceMeta, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return nil, r.meta(entity.SourceStateFailed, err), err
	}
	n, _ := strconv.ParseInt(uid, 10, 64)
	a := &args{}
	from := r.usersFrom(a)
	sql := fmt.Sprintf("SELECT %s %s WHERE u.id = %s LIMIT 1", userSelectCols, from, a.add(n))
	rows, qerr := r.query(ctx, sql, a)
	if qerr != nil {
		return nil, r.meta(classifyErr(qerr), qerr), fmt.Errorf("get octop user: %w", qerr)
	}
	if len(rows) == 0 {
		return nil, r.meta(entity.SourceStateReady, nil), ErrNotFound
	}
	return r.mapUser(rows[0]), r.meta(entity.SourceStateReady, nil), nil
}

func (r *Reader) mapUser(row map[string]any) *entity.BizUser {
	asOf := r.now()
	def := "users.id 对应的 agents(kind=expert) / threads 分组计数"
	u := &entity.BizUser{
		UserId:      stringify(row["user_id"]),
		Username:    stringify(row["username"]),
		DisplayName: stringify(row["display_name"]),
		Enabled:     toEnabled(row["enabled"]),
		RoleSummary: stringify(row["role"]),
		CreatedAt:   toTime(row["created_at"]),
		LastActive:  toTime(row["last_active"]),
	}
	if v, ok := toInt64(row["expert_count"]); ok {
		u.ExpertCount = entity.NewStatOK(v, asOf, def)
	} else {
		u.ExpertCount = entity.NewStatNull(entity.SourceStateFailed, &asOf)
	}
	if v, ok := toInt64(row["thread_count"]); ok {
		u.ThreadCount = entity.NewStatOK(v, asOf, def)
	} else {
		u.ThreadCount = entity.NewStatNull(entity.SourceStateFailed, &asOf)
	}
	return u
}

// ---------------------------------------------------------------- counts

const (
	defExperts       = "agents 中 kind=expert 且 user_id 匹配的去重 agent_id 数"
	defThreads       = "threads 中 user_id 匹配的去重 thread_id 数（含空会话）"
	defTemplates     = "published_experts 中 created_by = 用户 ID 的记录数"
	defSkillPackages = "skill_packages 中 created_by = 用户 ID 的记录数（仅本人创建）"
)

// CountExperts 用户专家数。
func (r *Reader) CountExperts(ctx context.Context, userId string) (entity.StatValue, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return r.failedStat(entity.SourceStateFailed), err
	}
	a := &args{}
	sql := fmt.Sprintf("SELECT COUNT(DISTINCT agent_id) AS cnt FROM %s WHERE user_id::text = %s AND kind = %s",
		quoteIdent(r.tables.Agents), a.add(uid), a.add(kindExpert))
	return r.count(ctx, sql, a, defExperts)
}

// CountThreads 用户会话数（含空会话）。
func (r *Reader) CountThreads(ctx context.Context, userId string) (entity.StatValue, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return r.failedStat(entity.SourceStateFailed), err
	}
	a := &args{}
	sql := fmt.Sprintf("SELECT COUNT(DISTINCT thread_id) AS cnt FROM %s WHERE user_id::text = %s",
		quoteIdent(r.tables.Threads), a.add(uid))
	return r.count(ctx, sql, a, defThreads)
}

// CountTemplates 用户发布的专家模板数。
func (r *Reader) CountTemplates(ctx context.Context, userId string) (entity.StatValue, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return r.failedStat(entity.SourceStateFailed), err
	}
	a := &args{}
	sql := fmt.Sprintf("SELECT COUNT(DISTINCT published_expert_id) AS cnt FROM %s WHERE created_by = %s",
		quoteIdent(r.tables.PublishedExperts), a.add(uid))
	return r.count(ctx, sql, a, defTemplates)
}

// CountSkillPackages 用户自建技能包数。
func (r *Reader) CountSkillPackages(ctx context.Context, userId string) (entity.StatValue, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return r.failedStat(entity.SourceStateFailed), err
	}
	a := &args{}
	sql := fmt.Sprintf("SELECT COUNT(DISTINCT skill_package_id) AS cnt FROM %s WHERE created_by = %s",
		quoteIdent(r.tables.SkillPackages), a.add(uid))
	return r.count(ctx, sql, a, defSkillPackages)
}

func (r *Reader) count(ctx context.Context, sql string, a *args, def string) (entity.StatValue, error) {
	rows, err := r.query(ctx, sql, a)
	asOf := r.now()
	if err != nil {
		return entity.NewStatNull(classifyErr(err), &asOf), fmt.Errorf("count octop business: %w", err)
	}
	var v int64
	if len(rows) > 0 {
		n, ok := toInt64(rows[0]["cnt"])
		if !ok {
			e := errors.New("unexpected count result")
			return entity.NewStatNull(entity.SourceStateFailed, &asOf), e
		}
		v = n
	}
	return entity.NewStatOK(v, asOf, def), nil
}

func (r *Reader) failedStat(state string) entity.StatValue {
	asOf := r.now()
	return entity.NewStatNull(state, &asOf)
}

// ---------------------------------------------------------------- experts

const expertListCols = `agent_id AS expert_id, name AS name, description AS description, default_model AS model, ` +
	`enabled AS enabled, last_state AS last_state, COALESCE(NULLIF(published_expert_id, ''), template_name) AS source_template, ` +
	`created_at AS created_at, updated_at AS updated_at`

// PageExperts 用户专家分页（不含 config）。
func (r *Reader) PageExperts(ctx context.Context, userId string, q *entity.BusinessQuery) (*model.PageResult[*entity.BizExpert], *entity.SourceMeta, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return nil, r.meta(entity.SourceStateFailed, err), err
	}
	q = normalizeQuery(q)
	a := &args{}
	conds := []string{"user_id::text = " + a.add(uid), "kind = " + a.add(kindExpert)}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		conds = append(conds, fmt.Sprintf(`name ILIKE %s ESCAPE '\'`, a.add(likePattern(kw))))
	}
	if q.Enabled != nil {
		conds = append(conds, "enabled = "+a.add(enabledFlag(*q.Enabled)))
	}
	if m := strings.TrimSpace(q.Model); m != "" {
		conds = append(conds, "default_model = "+a.add(m))
	}
	table := quoteIdent(r.tables.Agents)
	where := whereClause(conds)
	countSQL := fmt.Sprintf("SELECT COUNT(DISTINCT agent_id) AS total FROM %s%s", table, where)
	listSQL := fmt.Sprintf("SELECT %s FROM %s%s ORDER BY created_at DESC, agent_id ASC", expertListCols, table, where)
	ca := &args{vals: append([]any(nil), a.vals...)}
	return runPage(ctx, r, countSQL, ca, listSQL, a, q, r.mapExpert)
}

// GetExpert 单个专家（含白名单配置投影）。
func (r *Reader) GetExpert(ctx context.Context, userId, expertId string) (*entity.BizExpert, *entity.SourceMeta, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return nil, r.meta(entity.SourceStateFailed, err), err
	}
	a := &args{}
	sql := fmt.Sprintf("SELECT %s, config_json AS config, skill_package_ids, knowledge_base_ids, mcp_servers FROM %s WHERE agent_id = %s AND user_id::text = %s AND kind = %s LIMIT 1",
		expertListCols, quoteIdent(r.tables.Agents), a.add(expertId), a.add(uid), a.add(kindExpert))
	rows, qerr := r.query(ctx, sql, a)
	if qerr != nil {
		return nil, r.meta(classifyErr(qerr), qerr), fmt.Errorf("get octop expert: %w", qerr)
	}
	if len(rows) == 0 {
		return nil, r.meta(entity.SourceStateReady, nil), ErrNotFound
	}
	e := r.mapExpert(rows[0])
	e.Config = projectStoredConfig(rows[0], e)
	if e.Config != nil && hasConfigKey(e.Config, "skill_packages") {
		n := int64(len(e.Config.SkillPackages))
		e.SkillPkgCount = &n
	}
	return e, r.meta(entity.SourceStateReady, nil), nil
}

func (r *Reader) mapExpert(row map[string]any) *entity.BizExpert {
	return &entity.BizExpert{
		ExpertId:       stringify(row["expert_id"]),
		Name:           stringify(row["name"]),
		Description:    stringify(row["description"]),
		Model:          stringify(row["model"]),
		Enabled:        toEnabled(row["enabled"]),
		LastState:      stringify(row["last_state"]),
		SourceTemplate: stringify(row["source_template"]),
		CreatedAt:      toTime(row["created_at"]),
		UpdatedAt:      toTime(row["updated_at"]),
	}
}

// configWhitelist 配置 DTO 的白名单键，源字段由 projectStoredConfig 映射。
var configWhitelist = []string{
	"model_ref", "provider_name", "provider_id",
	"temperature", "top_p", "max_tokens", "max_iterations", "input_length", "trajectory_on",
	"skill_packages", "knowledge_bases", "mcp_connections", "sub_experts",
}

// projectStoredConfig 对齐 Octop 的运行参数及独立挂载列，保持现有 API 字段。
// 独立列优先；未设置时兼容旧 config_json 中的挂载 ID 列表，不补默认值。
func projectStoredConfig(row map[string]any, e *entity.BizExpert) *entity.BizExpertConfig {
	raw := parseJSONObject(row["config"])
	projected := make(map[string]any, len(configWhitelist))
	for _, key := range configWhitelist {
		if value, ok := raw[key]; ok {
			projected[key] = value
		}
	}
	for _, mapping := range [][2]string{
		{"max_iters", "max_iterations"}, {"max_input_length", "input_length"},
		{"enable_trajectory", "trajectory_on"},
	} {
		if value, ok := raw[mapping[0]]; ok {
			projected[mapping[1]] = value
		}
	}
	if e.Model != "" {
		projected["model_ref"] = e.Model
	}
	for _, mapping := range [][2]string{
		{"skill_package_ids", "skill_packages"}, {"knowledge_base_ids", "knowledge_bases"},
		{"mcp_servers", "mcp_connections"},
	} {
		if value, ok := raw[mapping[0]]; ok {
			projected[mapping[1]] = value
		}
		if value := parseStoredIDList(row[mapping[0]]); value != nil {
			projected[mapping[1]] = value
		}
	}
	return projectConfig(projected, e)
}

func parseStoredIDList(raw any) []any {
	var data []byte
	switch value := raw.(type) {
	case string:
		data = []byte(value)
	case []byte:
		data = value
	default:
		return nil
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil || ids == nil {
		return nil
	}
	values := make([]any, 0, len(ids))
	for _, id := range ids {
		if strings.TrimSpace(id) != "" {
			values = append(values, id)
		}
	}
	return values
}

// projectConfig 仅投影白名单键；缺失键写入 Unsupported，绝不补默认值。
// description/enabled/last_state 来自列而非 JSON，直接复用已读取的列值。
func projectConfig(raw any, e *entity.BizExpert) *entity.BizExpertConfig {
	cfg := &entity.BizExpertConfig{
		Description: e.Description,
		Enabled:     e.Enabled,
		LastState:   e.LastState,
	}
	m := parseJSONObject(raw)
	for _, key := range configWhitelist {
		v, ok := m[key]
		if !ok || v == nil {
			cfg.Unsupported = append(cfg.Unsupported, key)
			continue
		}
		switch key {
		case "model_ref":
			cfg.ModelRef = stringify(v)
		case "provider_name":
			cfg.ProviderName = stringify(v)
		case "provider_id":
			cfg.ProviderId = stringify(v)
		case "temperature":
			cfg.Temperature = toFloatPtr(v)
		case "top_p":
			cfg.TopP = toFloatPtr(v)
		case "max_tokens":
			cfg.MaxTokens = toInt64Ptr(v)
		case "max_iterations":
			cfg.MaxIterations = toInt64Ptr(v)
		case "input_length":
			cfg.InputLength = toInt64Ptr(v)
		case "trajectory_on":
			if b, ok := v.(bool); ok {
				cfg.TrajectoryOn = &b
			} else {
				cfg.Unsupported = append(cfg.Unsupported, key)
			}
		case "skill_packages":
			cfg.SkillPackages = toRefs(v)
		case "knowledge_bases":
			cfg.KnowledgeBases = toRefs(v)
		case "mcp_connections":
			cfg.MCPConnections = toRefs(v)
		case "sub_experts":
			cfg.SubExperts = toRefs(v)
		}
	}
	return cfg
}

// hasConfigKey 判断配置键是否由 JSON 提供（未出现在 Unsupported 中）。
func hasConfigKey(cfg *entity.BizExpertConfig, key string) bool {
	return !containsStr(cfg.Unsupported, key)
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func parseJSONObject(raw any) map[string]any {
	switch t := raw.(type) {
	case nil:
		return nil
	case map[string]any:
		return t
	case []byte:
		return unmarshalObject(t)
	case string:
		return unmarshalObject([]byte(t))
	default:
		return nil
	}
}

func unmarshalObject(b []byte) map[string]any {
	if len(strings.TrimSpace(string(b))) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

func toRefs(v any) []entity.BizRefSummary {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]entity.BizRefSummary, 0, len(list))
	for _, it := range list {
		switch t := it.(type) {
		case string:
			out = append(out, entity.BizRefSummary{Id: t})
		case map[string]any:
			ref := entity.BizRefSummary{Id: stringify(t["id"]), Name: stringify(t["name"]), Status: stringify(t["status"])}
			if ref.Id != "" || ref.Name != "" {
				out = append(out, ref)
			}
		}
	}
	return out
}

// ---------------------------------------------------------------- threads

// PageThreads 用户会话分页。按 threads.user_id 过滤；LEFT JOIN agents 仅取名称，
// agent 被删除时不丢会话。agents.agent_id 假定唯一，故 JOIN 不会放大行数。
func (r *Reader) PageThreads(ctx context.Context, userId string, q *entity.BusinessQuery) (*model.PageResult[*entity.BizThread], *entity.SourceMeta, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return nil, r.meta(entity.SourceStateFailed, err), err
	}
	q = normalizeQuery(q)
	a := &args{}
	conds, err := r.threadConds(a, uid, q)
	if err != nil {
		return nil, r.meta(entity.SourceStateFailed, err), err
	}
	where := whereClause(conds)
	threads := quoteIdent(r.tables.Threads)
	countSQL := fmt.Sprintf("SELECT COUNT(DISTINCT t.thread_id) AS total FROM %s t%s", threads, where)
	listSQL := fmt.Sprintf("SELECT %s FROM %s t LEFT JOIN %s a ON a.agent_id = t.agent_id%s ORDER BY t.last_active DESC, t.created_at DESC, t.thread_id ASC",
		threadCols, threads, quoteIdent(r.tables.Agents), where)
	ca := &args{vals: append([]any(nil), a.vals...)}
	return runPage(ctx, r, countSQL, ca, listSQL, a, q, r.mapThread)
}

const threadCols = `t.thread_id AS thread_id, t.title AS title, t.agent_id AS agent_id, a.name AS agent_name, ` +
	`t.channel_type AS channel, t.created_at AS created_at, t.last_active AS last_active`

func (r *Reader) threadConds(a *args, uid string, q *entity.BusinessQuery) ([]string, error) {
	conds := []string{"t.user_id::text = " + a.add(uid)}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		conds = append(conds, fmt.Sprintf(`t.title ILIKE %s ESCAPE '\'`, a.add(likePattern(kw))))
	}
	if c := strings.TrimSpace(q.Channel); c != "" {
		conds = append(conds, "t.channel_type = "+a.add(c))
	}
	if ag := strings.TrimSpace(q.AgentId); ag != "" {
		conds = append(conds, "t.agent_id = "+a.add(ag))
	}
	col := ""
	switch strings.TrimSpace(q.TimeField) {
	case "", "created":
		col = "t.created_at"
	case "last_active":
		col = "t.last_active"
	default:
		return nil, fmt.Errorf("%w: timeField %q", ErrInvalidQuery, q.TimeField)
	}
	if s := strings.TrimSpace(q.StartTime); s != "" {
		v, ok := parseFilterTime(s)
		if !ok {
			return nil, fmt.Errorf("%w: startTime", ErrInvalidQuery)
		}
		conds = append(conds, col+" >= "+a.add(v))
	}
	if s := strings.TrimSpace(q.EndTime); s != "" {
		v, ok := parseFilterTime(s)
		if !ok {
			return nil, fmt.Errorf("%w: endTime", ErrInvalidQuery)
		}
		conds = append(conds, col+" <= "+a.add(v))
	}
	return conds, nil
}

// GetThread 单个会话。
func (r *Reader) GetThread(ctx context.Context, userId, threadId string) (*entity.BizThread, *entity.SourceMeta, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return nil, r.meta(entity.SourceStateFailed, err), err
	}
	a := &args{}
	sql := fmt.Sprintf("SELECT %s FROM %s t LEFT JOIN %s a ON a.agent_id = t.agent_id WHERE t.thread_id = %s AND t.user_id::text = %s LIMIT 1",
		threadCols, quoteIdent(r.tables.Threads), quoteIdent(r.tables.Agents), a.add(threadId), a.add(uid))
	rows, qerr := r.query(ctx, sql, a)
	if qerr != nil {
		return nil, r.meta(classifyErr(qerr), qerr), fmt.Errorf("get octop thread: %w", qerr)
	}
	if len(rows) == 0 {
		return nil, r.meta(entity.SourceStateReady, nil), ErrNotFound
	}
	return r.mapThread(rows[0]), r.meta(entity.SourceStateReady, nil), nil
}

func (r *Reader) mapThread(row map[string]any) *entity.BizThread {
	last := toTime(row["last_active"]) // 0 => nil => 无消息
	return &entity.BizThread{
		ThreadId:    stringify(row["thread_id"]),
		Title:       stringify(row["title"]),
		AgentId:     stringify(row["agent_id"]),
		AgentName:   stringify(row["agent_name"]),
		Channel:     stringify(row["channel"]),
		CreatedAt:   toTime(row["created_at"]),
		LastActive:  last,
		HasMessages: last != nil,
	}
}

// ---------------------------------------------------------------- templates

const templateCols = `pe.published_expert_id AS template_id, pe.slug AS slug, pe.name AS name, pe.description AS description, ` +
	`pe.created_by AS created_by, pe.source_agent_id AS source_expert_id, ` +
	`(ag.agent_id IS NOT NULL) AS source_available, pe.created_at AS created_at, pe.updated_at AS updated_at`

func (r *Reader) templateFrom() string {
	return fmt.Sprintf("FROM %s pe LEFT JOIN %s ag ON ag.agent_id = pe.source_agent_id",
		quoteIdent(r.tables.PublishedExperts), quoteIdent(r.tables.Agents))
}

// PageExpertTemplates 用户发布的模板：始终 created_by = userId。
func (r *Reader) PageExpertTemplates(ctx context.Context, userId string, q *entity.BusinessQuery) (*model.PageResult[*entity.BizExpertTemplate], *entity.SourceMeta, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return nil, r.meta(entity.SourceStateFailed, err), err
	}
	q = normalizeQuery(q)
	a := &args{}
	conds := []string{"pe.created_by = " + a.add(uid)}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		conds = append(conds, fmt.Sprintf(`pe.name ILIKE %s ESCAPE '\'`, a.add(likePattern(kw))))
	}
	where := whereClause(conds)
	countSQL := fmt.Sprintf("SELECT COUNT(DISTINCT pe.published_expert_id) AS total FROM %s pe%s", quoteIdent(r.tables.PublishedExperts), where)
	listSQL := fmt.Sprintf("SELECT %s %s%s ORDER BY pe.updated_at DESC, pe.published_expert_id ASC", templateCols, r.templateFrom(), where)
	ca := &args{vals: append([]any(nil), a.vals...)}
	return runPage(ctx, r, countSQL, ca, listSQL, a, q, r.mapTemplate)
}

// GetExpertTemplate 单个模板（必须属于该用户）。
func (r *Reader) GetExpertTemplate(ctx context.Context, userId, templateId string) (*entity.BizExpertTemplate, *entity.SourceMeta, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return nil, r.meta(entity.SourceStateFailed, err), err
	}
	a := &args{}
	sql := fmt.Sprintf("SELECT %s %s WHERE pe.published_expert_id = %s AND pe.created_by = %s LIMIT 1",
		templateCols, r.templateFrom(), a.add(templateId), a.add(uid))
	rows, qerr := r.query(ctx, sql, a)
	if qerr != nil {
		return nil, r.meta(classifyErr(qerr), qerr), fmt.Errorf("get octop template: %w", qerr)
	}
	if len(rows) == 0 {
		return nil, r.meta(entity.SourceStateReady, nil), ErrNotFound
	}
	return r.mapTemplate(rows[0]), r.meta(entity.SourceStateReady, nil), nil
}

func (r *Reader) mapTemplate(row map[string]any) *entity.BizExpertTemplate {
	t := &entity.BizExpertTemplate{
		TemplateId:     stringify(row["template_id"]),
		Slug:           stringify(row["slug"]),
		Name:           stringify(row["name"]),
		Description:    stringify(row["description"]),
		CreatedBy:      stringify(row["created_by"]),
		SourceExpertId: stringify(row["source_expert_id"]),
		CreatedAt:      toTime(row["created_at"]),
		UpdatedAt:      toTime(row["updated_at"]),
	}
	if t.SourceExpertId != "" {
		if b, ok := row["source_available"].(bool); ok {
			t.SourceAvailable = &b
		}
	}
	return t
}

// ---------------------------------------------------------------- skill packages

const packageCols = `sp.skill_package_id AS package_id, sp.name AS name, sp.description AS description, ` +
	`sp.created_by AS created_by, sp.skill_count AS skill_count, sp.shared AS shared, sp.updated_at AS updated_at`

// PageSkillPackages 用户自建技能包：始终 created_by = userId（不含共享给该用户的）。
func (r *Reader) PageSkillPackages(ctx context.Context, userId string, q *entity.BusinessQuery) (*model.PageResult[*entity.BizSkillPackage], *entity.SourceMeta, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return nil, r.meta(entity.SourceStateFailed, err), err
	}
	q = normalizeQuery(q)
	a := &args{}
	conds := []string{"sp.created_by = " + a.add(uid)}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		conds = append(conds, fmt.Sprintf(`sp.name ILIKE %s ESCAPE '\'`, a.add(likePattern(kw))))
	}
	table := quoteIdent(r.tables.SkillPackages)
	where := whereClause(conds)
	countSQL := fmt.Sprintf("SELECT COUNT(DISTINCT sp.skill_package_id) AS total FROM %s sp%s", table, where)
	listSQL := fmt.Sprintf("SELECT %s FROM %s sp%s ORDER BY sp.updated_at DESC, sp.skill_package_id ASC", packageCols, table, where)
	ca := &args{vals: append([]any(nil), a.vals...)}
	return runPage(ctx, r, countSQL, ca, listSQL, a, q, r.mapPackage)
}

// GetSkillPackage 单个技能包（必须 created_by = userId）。
func (r *Reader) GetSkillPackage(ctx context.Context, userId, packageId string) (*entity.BizSkillPackage, *entity.SourceMeta, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return nil, r.meta(entity.SourceStateFailed, err), err
	}
	a := &args{}
	sql := fmt.Sprintf("SELECT %s FROM %s sp WHERE sp.skill_package_id = %s AND sp.created_by = %s LIMIT 1",
		packageCols, quoteIdent(r.tables.SkillPackages), a.add(packageId), a.add(uid))
	rows, qerr := r.query(ctx, sql, a)
	if qerr != nil {
		return nil, r.meta(classifyErr(qerr), qerr), fmt.Errorf("get octop skill package: %w", qerr)
	}
	if len(rows) == 0 {
		return nil, r.meta(entity.SourceStateReady, nil), ErrNotFound
	}
	return r.mapPackage(rows[0]), r.meta(entity.SourceStateReady, nil), nil
}

func (r *Reader) mapPackage(row map[string]any) *entity.BizSkillPackage {
	p := &entity.BizSkillPackage{
		PackageId:   stringify(row["package_id"]),
		Name:        stringify(row["name"]),
		Description: stringify(row["description"]),
		CreatedBy:   stringify(row["created_by"]),
		UpdatedAt:   toTime(row["updated_at"]),
	}
	asOf := r.now()
	if v, ok := toInt64(row["skill_count"]); ok {
		p.SkillCount = entity.NewStatOK(v, asOf, "skill_packages.skill_count")
	} else {
		p.SkillCount = entity.NewStatNull(entity.SourceStateUnsupported, &asOf)
	}
	p.Shared = toBool(row["shared"])
	return p
}

// ---------------------------------------------------------------- skills

const skillCols = `s.slug AS slug, s.name AS name, s.description AS description, s.source AS source`

// skillFrom 通过 JOIN skill_packages 强制 created_by = userId，防止越权读取他人包内技能。
func (r *Reader) skillFrom() string {
	return fmt.Sprintf("FROM %s s JOIN %s sp ON sp.skill_package_id = s.skill_package_id",
		quoteIdent(r.tables.Skills), quoteIdent(r.tables.SkillPackages))
}

// PageSkills 技能包内技能分页。
func (r *Reader) PageSkills(ctx context.Context, userId, packageId string, q *entity.BusinessQuery) (*model.PageResult[*entity.BizSkill], *entity.SourceMeta, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return nil, r.meta(entity.SourceStateFailed, err), err
	}
	q = normalizeQuery(q)
	a := &args{}
	conds := []string{"sp.created_by = " + a.add(uid), "sp.skill_package_id = " + a.add(packageId)}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		conds = append(conds, fmt.Sprintf(`s.name ILIKE %s ESCAPE '\'`, a.add(likePattern(kw))))
	}
	where := whereClause(conds)
	countSQL := fmt.Sprintf("SELECT COUNT(DISTINCT s.slug) AS total %s%s", r.skillFrom(), where)
	listSQL := fmt.Sprintf("SELECT %s %s%s ORDER BY s.slug ASC", skillCols, r.skillFrom(), where)
	ca := &args{vals: append([]any(nil), a.vals...)}
	return runPage(ctx, r, countSQL, ca, listSQL, a, q, r.mapSkill)
}

// GetSkill 单个技能。
func (r *Reader) GetSkill(ctx context.Context, userId, packageId, slug string) (*entity.BizSkill, *entity.SourceMeta, error) {
	uid, err := normalizeUserId(userId)
	if err != nil {
		return nil, r.meta(entity.SourceStateFailed, err), err
	}
	a := &args{}
	sql := fmt.Sprintf("SELECT %s %s WHERE sp.created_by = %s AND sp.skill_package_id = %s AND s.slug = %s LIMIT 1",
		skillCols, r.skillFrom(), a.add(uid), a.add(packageId), a.add(slug))
	rows, qerr := r.query(ctx, sql, a)
	if qerr != nil {
		return nil, r.meta(classifyErr(qerr), qerr), fmt.Errorf("get octop skill: %w", qerr)
	}
	if len(rows) == 0 {
		return nil, r.meta(entity.SourceStateReady, nil), ErrNotFound
	}
	return r.mapSkill(rows[0]), r.meta(entity.SourceStateReady, nil), nil
}

func (r *Reader) mapSkill(row map[string]any) *entity.BizSkill {
	return &entity.BizSkill{
		Slug:        stringify(row["slug"]),
		Name:        stringify(row["name"]),
		Description: stringify(row["description"]),
		Source:      stringify(row["source"]),
	}
}

// ---------------------------------------------------------------- shared helpers

type args struct{ vals []any }

func (a *args) add(v any) string {
	a.vals = append(a.vals, v)
	return "$" + strconv.Itoa(len(a.vals))
}

func whereClause(conds []string) string {
	if len(conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conds, " AND ")
}

func (r *Reader) query(ctx context.Context, sql string, a *args) ([]map[string]any, error) {
	_, rows, err := r.q.QueryContext(ctx, sql, a.vals...)
	return rows, err
}

func (r *Reader) meta(state string, err error) *entity.SourceMeta {
	now := r.now()
	m := &entity.SourceMeta{Source: SourceName, State: state, ReadAt: &now}
	if state == entity.SourceStateReady {
		m.LastSuccessAt = &now
		return m
	}
	switch state {
	case entity.SourceStateUnsupported:
		m.Message = "octop business table or column not available"
	case entity.SourceStateForbidden:
		m.Message = "permission denied reading octop business data"
	default:
		m.Message = "octop business read failed"
		if err != nil && (errors.Is(err, ErrInvalidUserId) || errors.Is(err, ErrInvalidQuery)) {
			m.Message = err.Error()
		}
	}
	return m
}

// runPage 先 COUNT 再取页；页大小 [1,100]，默认 10。
func runPage[T any](ctx context.Context, r *Reader, countSQL string, ca *args, listSQL string, la *args, q *entity.BusinessQuery, mapRow func(map[string]any) T) (*model.PageResult[T], *entity.SourceMeta, error) {
	rows, err := r.query(ctx, countSQL, ca)
	if err != nil {
		return nil, r.meta(classifyErr(err), err), fmt.Errorf("count octop business page: %w", err)
	}
	var total int64
	if len(rows) > 0 {
		n, ok := toInt64(rows[0]["total"])
		if !ok {
			e := errors.New("unexpected total count result")
			return nil, r.meta(entity.SourceStateFailed, e), e
		}
		total = n
	}
	res := &model.PageResult[T]{Total: total, List: make([]T, 0), PageNum: q.PageNum, PageSize: q.PageSize}
	if total == 0 {
		return res, r.meta(entity.SourceStateReady, nil), nil
	}
	offset := (q.PageNum - 1) * q.PageSize
	sql := listSQL + " LIMIT " + la.add(q.PageSize) + " OFFSET " + la.add(offset)
	list, err := r.query(ctx, sql, la)
	if err != nil {
		return nil, r.meta(classifyErr(err), err), fmt.Errorf("list octop business page: %w", err)
	}
	for _, row := range list {
		res.List = append(res.List, mapRow(row))
	}
	return res, r.meta(entity.SourceStateReady, nil), nil
}

func normalizeQuery(q *entity.BusinessQuery) *entity.BusinessQuery {
	c := entity.BusinessQuery{}
	if q != nil {
		c = *q
	}
	if c.PageNum <= 0 {
		c.PageNum = 1
	}
	if c.PageSize <= 0 {
		c.PageSize = defaultPageSize
	}
	if c.PageSize > maxPageSize {
		c.PageSize = maxPageSize
	}
	return &c
}

// normalizeUserId 仅接受规范十进制非负整数（无前导零/符号/空白）。
func normalizeUserId(s string) (string, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 || strconv.FormatInt(n, 10) != s {
		return "", fmt.Errorf("%w: %q", ErrInvalidUserId, s)
	}
	return s, nil
}

// classifyErr 将底层错误分类为 SourceMeta.State。
func classifyErr(err error) string {
	if err == nil {
		return entity.SourceStateReady
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "permission denied"),
		strings.Contains(msg, "insufficientprivilege"),
		strings.Contains(msg, "insufficient_privilege"),
		strings.Contains(msg, "access denied"):
		return entity.SourceStateForbidden
	case strings.Contains(msg, "does not exist"),
		strings.Contains(msg, "undefined_table"),
		strings.Contains(msg, "undefined_column"),
		strings.Contains(msg, "unknown table"):
		return entity.SourceStateUnsupported
	default:
		return entity.SourceStateFailed
	}
}

func quoteIdent(name string) string {
	n := strings.ReplaceAll(name, `"`, ``)
	parts := strings.Split(n, ".")
	for i, p := range parts {
		parts[i] = `"` + p + `"`
	}
	return strings.Join(parts, ".")
}

// likePattern 转义 LIKE 通配符，配合 ESCAPE '\'。
func likePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return "%" + s + "%"
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func toInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case uint64:
		return int64(t), true
	case float64:
		return int64(t), true
	case []byte:
		n, err := strconv.ParseInt(strings.TrimSpace(string(t)), 10, 64)
		return n, err == nil
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n, err == nil
	}
	return 0, false
}

func toInt64Ptr(v any) *int64 {
	f, ok := v.(float64)
	if !ok {
		if n, ok2 := toInt64(v); ok2 {
			return &n
		}
		return nil
	}
	n := int64(f)
	return &n
}

func toFloatPtr(v any) *float64 {
	switch t := v.(type) {
	case float64:
		return &t
	case int64:
		f := float64(t)
		return &f
	case int:
		f := float64(t)
		return &f
	}
	return nil
}

func toBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case int64:
		return t != 0
	case int:
		return t != 0
	case string:
		b, _ := strconv.ParseBool(t)
		return b
	}
	return false
}

// enabledFlag 源端 enabled/disabled 均为整数标记，不能传 PostgreSQL 布尔参数。
func enabledFlag(enabled int8) int {
	if enabled > 0 {
		return 1
	}
	return 0
}

// toEnabled 1 启用 / -1 禁用 / 0 未知。
func toEnabled(v any) int8 {
	switch t := v.(type) {
	case nil:
		return 0
	case bool:
		if t {
			return 1
		}
		return -1
	}
	if n, ok := toInt64(v); ok {
		if n != 0 {
			return 1
		}
		return -1
	}
	return 0
}

// toTime 兼容 time.Time、epoch 秒/毫秒（数值或数字串）、常见时间字符串。0 / 空 => nil。
func toTime(v any) *time.Time {
	switch t := v.(type) {
	case nil:
		return nil
	case time.Time:
		if t.IsZero() {
			return nil
		}
		return &t
	case *time.Time:
		return t
	case float64:
		return epochToTime(t)
	case int64:
		return epochToTime(float64(t))
	case int:
		return epochToTime(float64(t))
	case []byte:
		return toTime(string(t))
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return epochToTime(f)
		}
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02"} {
			if tm, err := time.Parse(layout, s); err == nil {
				return &tm
			}
		}
	}
	return nil
}

func epochToTime(f float64) *time.Time {
	if f <= 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		return nil
	}
	if f > 1e11 { // 毫秒
		f /= 1000
	}
	sec := math.Floor(f)
	tm := time.Unix(int64(sec), int64((f-sec)*1e9)).UTC()
	return &tm
}

// parseFilterTime 过滤时间 => epoch 秒（与 threads 数值时间列比较）。
func parseFilterTime(s string) (float64, bool) {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if f > 1e11 {
			f /= 1000
		}
		return f, true
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02"} {
		if tm, err := time.Parse(layout, s); err == nil {
			return float64(tm.UnixNano()) / 1e9, true
		}
	}
	return 0, false
}
