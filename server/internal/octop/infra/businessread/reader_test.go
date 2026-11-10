package businessread

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"mayfly-go/internal/db/dbm/dbi"
	"mayfly-go/internal/octop/domain/entity"
)

type call struct {
	sql  string
	args []any
}

type route struct {
	contains string
	rows     []map[string]any
	err      error
}

// memQueryer 记录 SQL/参数，并按 contains 匹配返回预置结果；未匹配返回空结果。
type memQueryer struct {
	calls  []call
	routes []route
}

func (m *memQueryer) QueryContext(_ context.Context, sql string, args ...any) ([]*dbi.QueryColumn, []map[string]any, error) {
	m.calls = append(m.calls, call{sql: sql, args: args})
	for _, r := range m.routes {
		if strings.Contains(sql, r.contains) {
			return nil, r.rows, r.err
		}
	}
	return nil, nil, nil
}

func (m *memQueryer) last() call { return m.calls[len(m.calls)-1] }

var fixedNow = time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)

func newTestReader(q Queryer) *Reader {
	return NewReader(q, WithClock(func() time.Time { return fixedNow }))
}

func TestPageUsersCountsNotMultiplied(t *testing.T) {
	q := &memQueryer{routes: []route{
		{contains: "COUNT(*) AS total", rows: []map[string]any{{"total": int64(1)}}},
		{contains: "expert_count", rows: []map[string]any{{
			"user_id": int64(7), "username": "alice", "display_name": "Alice", "enabled": true,
			"role": "admin", "created_at": 1793872991.25,
			"expert_count": int64(3), "thread_count": int64(7), "last_active": int64(0),
		}}},
	}}
	res, meta, err := newTestReader(q).PageUsers(context.Background(), &entity.BusinessQuery{Keyword: "ali"})
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateReady, meta.State)
	require.Len(t, res.List, 1)
	u := res.List[0]
	require.Equal(t, "7", u.UserId)
	require.EqualValues(t, 3, *u.ExpertCount.Value)
	require.EqualValues(t, 7, *u.ThreadCount.Value) // 不是 3*7
	require.Nil(t, u.LastActive)
	require.Equal(t, int8(1), u.Enabled)

	list := q.last().sql
	require.NotContains(t, list, "SELECT *")
	// 两个独立分组子查询，且各自只引用一张表
	require.Equal(t, 2, strings.Count(list, "GROUP BY user_id::text"))
	require.Contains(t, list, `FROM "agents" WHERE kind = $1 GROUP BY user_id::text`)
	require.Contains(t, list, `FROM "threads" GROUP BY user_id::text`)
	require.NotContains(t, list, `"agents" a JOIN "threads"`)
	require.Equal(t, "expert", q.last().args[0])
}

func TestPageSizeClampedTo100(t *testing.T) {
	q := &memQueryer{routes: []route{
		{contains: "COUNT(*) AS total", rows: []map[string]any{{"total": int64(500)}}},
	}}
	res, _, err := newTestReader(q).PageUsers(context.Background(), &entity.BusinessQuery{})
	require.NoError(t, err)
	require.Equal(t, 10, res.PageSize)
	require.Equal(t, 1, res.PageNum)

	q.calls = nil
	res, _, err = newTestReader(q).PageUsers(context.Background(), &entity.BusinessQuery{})
	require.NoError(t, err)

	big := &entity.BusinessQuery{}
	big.PageNum, big.PageSize = 3, 9999
	q.calls = nil
	res, _, err = newTestReader(q).PageUsers(context.Background(), big)
	require.NoError(t, err)
	require.Equal(t, 100, res.PageSize)
	lastArgs := q.last().args
	require.EqualValues(t, 100, lastArgs[len(lastArgs)-2])
	require.EqualValues(t, 200, lastArgs[len(lastArgs)-1])
	require.Equal(t, 9999, big.PageSize, "caller query must not be mutated")
}

func TestTemplatesAlwaysFilterCreatedBy(t *testing.T) {
	q := &memQueryer{routes: []route{
		{contains: "COUNT(DISTINCT pe.published_expert_id)", rows: []map[string]any{{"total": int64(1)}}},
		{contains: "template_id", rows: []map[string]any{{
			"template_id": "pe1", "name": "T", "created_by": "42",
			"source_expert_id": "a1", "source_available": false,
		}}},
	}}
	r := newTestReader(q)
	res, _, err := r.PageExpertTemplates(context.Background(), "42", &entity.BusinessQuery{})
	require.NoError(t, err)
	require.Len(t, res.List, 1)
	require.False(t, *res.List[0].SourceAvailable)
	for _, c := range q.calls {
		require.Contains(t, c.sql, "pe.created_by = $1")
		require.Equal(t, "42", c.args[0])
	}

	q.calls = nil
	_, _, err = r.GetExpertTemplate(context.Background(), "42", "pe1")
	require.NoError(t, err)
	require.Contains(t, q.last().sql, "pe.created_by = $2")
	require.Equal(t, []any{"pe1", "42"}, q.last().args)

	q.calls = nil
	_, err = r.CountTemplates(context.Background(), "42")
	require.NoError(t, err)
	require.Contains(t, q.last().sql, "created_by = $1")
}

func TestSkillPackagesAndSkillsFilterCreatedBy(t *testing.T) {
	q := &memQueryer{routes: []route{
		{contains: "COUNT(DISTINCT", rows: []map[string]any{{"total": int64(1), "cnt": int64(2)}}},
		{contains: "s.slug AS slug", rows: []map[string]any{{"slug": "s1", "name": "S"}}},
		{contains: "package_id", rows: []map[string]any{{"package_id": "p1", "name": "P", "created_by": "9", "skill_count": int64(4), "shared": true}}},
	}}
	r := newTestReader(q)
	pk, _, err := r.PageSkillPackages(context.Background(), "9", nil)
	require.NoError(t, err)
	require.EqualValues(t, 4, *pk.List[0].SkillCount.Value)
	require.True(t, pk.List[0].Shared)
	for _, c := range q.calls {
		require.Contains(t, c.sql, "sp.created_by = $1")
	}

	q.calls = nil
	sk, _, err := r.PageSkills(context.Background(), "9", "p1", nil)
	require.NoError(t, err)
	require.Equal(t, "s1", sk.List[0].Slug)
	for _, c := range q.calls {
		require.Contains(t, c.sql, "sp.created_by = $1")
		require.Contains(t, c.sql, "sp.skill_package_id = $2")
		require.Equal(t, []any{"9", "p1"}, c.args[:2])
	}

	q.calls = nil
	cnt, err := r.CountSkillPackages(context.Background(), "9")
	require.NoError(t, err)
	require.EqualValues(t, 2, *cnt.Value)
	require.Contains(t, q.last().sql, "created_by = $1")
}

func TestClassifyErrorsToStates(t *testing.T) {
	cases := []struct {
		err   error
		state string
	}{
		{errors.New(`pq: relation "users" does not exist`), entity.SourceStateUnsupported},
		{errors.New("pq: permission denied for table users"), entity.SourceStateForbidden},
		{errors.New("connection reset"), entity.SourceStateFailed},
	}
	for _, c := range cases {
		q := &memQueryer{routes: []route{{contains: "FROM", err: c.err}}}
		r := newTestReader(q)

		res, meta, err := r.PageUsers(context.Background(), nil)
		require.Error(t, err)
		require.Nil(t, res)
		require.Equal(t, c.state, meta.State)
		require.Equal(t, SourceName, meta.Source)

		stat, err := r.CountExperts(context.Background(), "1")
		require.Error(t, err)
		require.Nil(t, stat.Value, "failure must not become 0")
		require.Equal(t, c.state, stat.State)
	}
	require.Equal(t, entity.SourceStateReady, newTestReader(&memQueryer{}).meta(entity.SourceStateReady, nil).State)
}

func TestCountExperts(t *testing.T) {
	q := &memQueryer{routes: []route{{contains: "COUNT(DISTINCT agent_id)", rows: []map[string]any{{"cnt": int64(5)}}}}}
	stat, err := newTestReader(q).CountExperts(context.Background(), "12")
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateReady, stat.State)
	require.EqualValues(t, 5, *stat.Value)
	require.Contains(t, q.last().sql, "user_id::text = $1")
	require.Contains(t, q.last().sql, "kind = $2")
	require.Equal(t, []any{"12", "expert"}, q.last().args)
}

func TestCountThreadsIncludesEmptySessions(t *testing.T) {
	q := &memQueryer{routes: []route{{contains: "COUNT(DISTINCT thread_id)", rows: []map[string]any{{"cnt": int64(2)}}}}}
	_, err := newTestReader(q).CountThreads(context.Background(), "12")
	require.NoError(t, err)
	require.NotContains(t, q.last().sql, "last_active")
}

func TestInvalidUserIdRejectedWithoutQuery(t *testing.T) {
	q := &memQueryer{}
	r := newTestReader(q)
	for _, bad := range []string{"", "abc", "01", "+1", "-1", " 1", "1.0", "1 OR 1=1"} {
		_, err := r.CountExperts(context.Background(), bad)
		require.ErrorIs(t, err, ErrInvalidUserId, bad)
		_, _, err = r.PageThreads(context.Background(), bad, nil)
		require.ErrorIs(t, err, ErrInvalidUserId, bad)
	}
	require.Empty(t, q.calls)
}

func TestPageThreadsLeftJoinAgentsAndNoMessages(t *testing.T) {
	q := &memQueryer{routes: []route{
		{contains: "COUNT(DISTINCT t.thread_id)", rows: []map[string]any{{"total": int64(2)}}},
		{contains: "agent_name", rows: []map[string]any{
			{"thread_id": "t1", "title": "hi", "agent_id": "gone", "agent_name": nil, "channel": "web", "created_at": 1793872991.0, "last_active": int64(0)},
			{"thread_id": "t2", "title": "yo", "agent_id": "a1", "agent_name": "Bot", "channel": "web", "created_at": 1793872991.0, "last_active": 1793873000.5},
		}},
	}}
	bq := &entity.BusinessQuery{Channel: "web", TimeField: "last_active", StartTime: "2026-01-01"}
	res, _, err := newTestReader(q).PageThreads(context.Background(), "5", bq)
	require.NoError(t, err)
	require.Len(t, res.List, 2)
	require.False(t, res.List[0].HasMessages)
	require.Nil(t, res.List[0].LastActive)
	require.Equal(t, "", res.List[0].AgentName)
	require.True(t, res.List[1].HasMessages)
	require.Equal(t, "Bot", res.List[1].AgentName)

	list := q.last().sql
	require.Contains(t, list, `LEFT JOIN "agents" a ON a.agent_id = t.agent_id`)
	require.Contains(t, list, "t.user_id::text = $1")
	require.Contains(t, list, "t.last_active >= $3")
	require.Equal(t, "5", q.last().args[0])

	_, _, err = newTestReader(q).PageThreads(context.Background(), "5", &entity.BusinessQuery{TimeField: "bogus"})
	require.ErrorIs(t, err, ErrInvalidQuery)
}

func TestGetExpertProjectsWhitelistWithoutDefaults(t *testing.T) {
	cfg := `{"model_ref":"gpt-x","temperature":0.2,"max_tokens":4096,"skill_packages":[{"id":"p1","name":"P"},"p2"],"secret_token":"leak"}`
	q := &memQueryer{routes: []route{{contains: "config_json AS config", rows: []map[string]any{{
		"expert_id": "a1", "name": "E", "enabled": true, "last_state": "idle", "config": []byte(cfg),
	}}}}}
	e, meta, err := newTestReader(q).GetExpert(context.Background(), "3", "a1")
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateReady, meta.State)
	require.Equal(t, []any{"a1", "3", "expert"}, q.last().args)
	require.NotNil(t, e.Config)
	require.Equal(t, "gpt-x", e.Config.ModelRef)
	require.InDelta(t, 0.2, *e.Config.Temperature, 1e-9)
	require.EqualValues(t, 4096, *e.Config.MaxTokens)
	require.Len(t, e.Config.SkillPackages, 2)
	require.EqualValues(t, 2, *e.SkillPkgCount)
	// 缺失字段不补默认
	require.Nil(t, e.Config.TopP)
	require.Nil(t, e.Config.MaxIterations)
	require.Nil(t, e.Config.TrajectoryOn)
	require.ElementsMatch(t,
		[]string{"provider_name", "provider_id", "top_p", "max_iterations", "input_length", "trajectory_on", "knowledge_bases", "mcp_connections", "sub_experts"},
		e.Config.Unsupported)
	require.NotContains(t, e.Config.Unsupported, "secret_token")
}

func TestGetExpertNoConfigAllUnsupported(t *testing.T) {
	q := &memQueryer{routes: []route{{contains: "config_json AS config", rows: []map[string]any{{"expert_id": "a1", "name": "E", "config": nil}}}}}
	e, _, err := newTestReader(q).GetExpert(context.Background(), "3", "a1")
	require.NoError(t, err)
	require.Len(t, e.Config.Unsupported, len(configWhitelist))
	require.Nil(t, e.SkillPkgCount)
}

func TestGetNotFound(t *testing.T) {
	q := &memQueryer{}
	r := newTestReader(q)
	_, meta, err := r.GetExpert(context.Background(), "3", "nope")
	require.ErrorIs(t, err, ErrNotFound)
	require.Equal(t, entity.SourceStateReady, meta.State)
	_, _, err = r.GetUser(context.Background(), "3")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestNoSelectStarAndParameterized(t *testing.T) {
	q := &memQueryer{routes: []route{
		{contains: "total", rows: []map[string]any{{"total": int64(1)}}},
	}}
	r := newTestReader(q)
	ctx := context.Background()
	bq := &entity.BusinessQuery{Keyword: "x'; DROP TABLE users;--"}
	_, _, _ = r.PageUsers(ctx, bq)
	_, _, _ = r.PageUserOptions(ctx, bq)
	_, _, _ = r.PageExperts(ctx, "1", bq)
	_, _, _ = r.PageThreads(ctx, "1", bq)
	_, _, _ = r.PageExpertTemplates(ctx, "1", bq)
	_, _, _ = r.PageSkillPackages(ctx, "1", bq)
	_, _, _ = r.PageSkills(ctx, "1", "p", bq)
	_, _, _ = r.GetSkill(ctx, "1", "p", "s")
	_, _, _ = r.GetSkillPackage(ctx, "1", "p")
	_, _, _ = r.GetThread(ctx, "1", "t")
	require.NotEmpty(t, q.calls)
	for _, c := range q.calls {
		require.NotContains(t, c.sql, "SELECT *")
		require.NotContains(t, c.sql, "DROP TABLE")
		require.Contains(t, c.sql, "$1")
	}
}

func TestWithTablesOverride(t *testing.T) {
	q := &memQueryer{}
	r := NewReader(q, WithTables(Tables{Users: "octop.users"}))
	_, _, _ = r.GetUser(context.Background(), "1")
	require.Contains(t, q.last().sql, `"octop"."users" u`)
	require.Contains(t, q.last().sql, `"agents"`)
}
