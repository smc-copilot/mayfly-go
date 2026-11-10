package businessread

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/stretchr/testify/require"

	"mayfly-go/internal/db/dbm/dbi"
	"mayfly-go/internal/octop/domain/entity"
)

// Run real queries against the column names/types verified on Octop-99.64.
// SQLite is an isolated schema regression fixture, not a PostgreSQL substitute:
// only ::text and ILIKE are translated. PostgreSQL integer/boolean mismatches
// are rejected explicitly because SQLite would otherwise silently accept them.
type deployedSchemaQueryer struct {
	db *sql.DB
}

var textCast = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_.]*)::text`)

func (q *deployedSchemaQueryer) QueryContext(ctx context.Context, query string, args ...any) ([]*dbi.QueryColumn, []map[string]any, error) {
	for _, arg := range args {
		if _, ok := arg.(bool); ok {
			return nil, nil, fmt.Errorf("integer status column compared with boolean parameter")
		}
	}
	query = textCast.ReplaceAllString(query, "CAST($1 AS TEXT)")
	query = strings.ReplaceAll(query, "ILIKE", "LIKE")
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	var result []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		row := make(map[string]any, len(columns))
		for i, name := range columns {
			row[name] = values[i]
		}
		result = append(result, row)
	}
	return nil, result, rows.Err()
}

func deployedSchemaReader(t *testing.T) *Reader {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	// Names and integer status types come from information_schema.columns,
	// inspected read-only on 2026-11-10. No production data or credentials.
	statements := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT, display_name TEXT, disabled INTEGER, role TEXT, created_at INTEGER, password_hash TEXT)`,
		`CREATE TABLE agents (agent_id TEXT PRIMARY KEY, user_id INTEGER, kind TEXT, name TEXT, description TEXT, default_model TEXT, enabled INTEGER, last_state TEXT, config_json TEXT, template_name TEXT, published_expert_id TEXT, skill_package_ids TEXT, knowledge_base_ids TEXT, mcp_servers TEXT, created_at INTEGER, updated_at INTEGER)`,
		`CREATE TABLE threads (thread_id TEXT PRIMARY KEY, user_id INTEGER, agent_id TEXT, title TEXT, channel_type TEXT, created_at INTEGER, last_active INTEGER)`,
		`INSERT INTO users VALUES (1,'alice','Alice',0,'user',100,'password-must-not-leak'), (2,'bob','Bob',1,'user',100,'hidden'), (3,'carol','Carol',0,'user',100,'hidden')`,
		`INSERT INTO agents VALUES ('a1',1,'expert','Research','Research expert','provider/model-x',1,'idle','{"temperature":0.2,"max_iters":12,"max_input_length":8192,"enable_trajectory":false,"secret_token":"do-not-leak"}','builtin',NULL,'["p1","p2"]','["kb1"]','["mcp1"]',100,110)`,
		`INSERT INTO agents VALUES ('a2',1,'expert','Published','Published expert','provider/model-y',0,'stopped','{}',NULL,'pe1','[]','[]','[]',100,120)`,
		`INSERT INTO agents (agent_id,user_id,kind,name,enabled) VALUES ('team1',1,'team','Team',1), ('b1',2,'expert','Shared expert',1)`,
		`INSERT INTO threads VALUES ('empty',1,'a1',NULL,'web',100,0), ('active',1,'a1','Active','web',100,120), ('shared',1,'b1','Shared','im',100,130), ('deleted',1,'gone','Deleted expert','web',100,140), ('bob-thread',2,'b1','Bob','web',100,150)`,
	}
	for _, statement := range statements {
		_, err := db.ExecContext(context.Background(), statement)
		require.NoError(t, err)
	}
	return newTestReader(&deployedSchemaQueryer{db: db})
}

func TestDeployedSchemaUsers(t *testing.T) {
	r := deployedSchemaReader(t)
	ctx := context.Background()
	page, meta, err := r.PageUsers(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, entity.SourceStateReady, meta.State)
	require.EqualValues(t, 3, page.Total)
	require.Len(t, page.List, 3)
	alice := page.List[0]
	require.Equal(t, "1", alice.UserId)
	require.Equal(t, int8(1), alice.Enabled)
	require.EqualValues(t, 2, *alice.ExpertCount.Value, "exclude teams and other owners")
	require.EqualValues(t, 4, *alice.ThreadCount.Value, "include empty/shared/deleted-expert threads without join multiplication")
	require.EqualValues(t, 140, alice.LastActive.Unix())
	require.EqualValues(t, 0, *page.List[2].ExpertCount.Value)
	require.EqualValues(t, 0, *page.List[2].ThreadCount.Value)
	require.Nil(t, page.List[2].LastActive)
	bob, _, err := r.GetUser(ctx, "2")
	require.NoError(t, err)
	require.Equal(t, int8(-1), bob.Enabled)
	encoded, err := json.Marshal(page)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "password")
}

func TestDeployedSchemaStatusFilters(t *testing.T) {
	r := deployedSchemaReader(t)
	for _, tc := range []struct {
		name    string
		enabled int8
		total   int64
	}{
		{"enabled", 1, 2}, {"disabled", -1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := &entity.BusinessQuery{Enabled: &tc.enabled}
			page, _, err := r.PageUsers(context.Background(), query)
			require.NoError(t, err)
			require.Equal(t, tc.total, page.Total)
			options, _, err := r.PageUserOptions(context.Background(), query)
			require.NoError(t, err)
			require.Equal(t, tc.total, options.Total)
		})
	}
}

func TestDeployedSchemaExperts(t *testing.T) {
	r := deployedSchemaReader(t)
	ctx := context.Background()
	enabled := int8(1)
	page, _, err := r.PageExperts(ctx, "1", &entity.BusinessQuery{Enabled: &enabled, Model: "provider/model-x"})
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Equal(t, "provider/model-x", page.List[0].Model)
	require.Equal(t, "builtin", page.List[0].SourceTemplate)
	e, _, err := r.GetExpert(ctx, "1", "a1")
	require.NoError(t, err)
	require.Equal(t, "provider/model-x", e.Config.ModelRef)
	require.InDelta(t, 0.2, *e.Config.Temperature, 1e-9)
	require.NotNil(t, e.Config.MaxIterations)
	require.EqualValues(t, 12, *e.Config.MaxIterations)
	require.NotNil(t, e.Config.InputLength)
	require.EqualValues(t, 8192, *e.Config.InputLength)
	require.NotNil(t, e.Config.TrajectoryOn)
	require.False(t, *e.Config.TrajectoryOn)
	require.Equal(t, []entity.BizRefSummary{{Id: "p1"}, {Id: "p2"}}, e.Config.SkillPackages)
	require.Equal(t, []entity.BizRefSummary{{Id: "kb1"}}, e.Config.KnowledgeBases)
	require.Equal(t, []entity.BizRefSummary{{Id: "mcp1"}}, e.Config.MCPConnections)
	require.EqualValues(t, 2, *e.SkillPkgCount)
	encoded, err := json.Marshal(e)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret_token")
	require.NotContains(t, string(encoded), "do-not-leak")
	published, _, err := r.GetExpert(ctx, "1", "a2")
	require.NoError(t, err)
	require.Equal(t, "pe1", published.SourceTemplate)
	require.EqualValues(t, 0, *published.SkillPkgCount)
	_, _, err = r.GetExpert(ctx, "2", "a1")
	require.ErrorIs(t, err, ErrNotFound)
	enabled = -1
	page, _, err = r.PageExperts(ctx, "1", &entity.BusinessQuery{Enabled: &enabled})
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Equal(t, int8(-1), page.List[0].Enabled)
}

func TestDeployedSchemaThreads(t *testing.T) {
	r := deployedSchemaReader(t)
	ctx := context.Background()
	page, _, err := r.PageThreads(ctx, "1", &entity.BusinessQuery{Channel: "web"})
	require.NoError(t, err)
	require.EqualValues(t, 3, page.Total)
	thread, _, err := r.GetThread(ctx, "1", "shared")
	require.NoError(t, err)
	require.Equal(t, "im", thread.Channel)
	require.Equal(t, "b1", thread.AgentId)
	deleted, _, err := r.GetThread(ctx, "1", "deleted")
	require.NoError(t, err)
	require.Empty(t, deleted.AgentName)
	empty, _, err := r.GetThread(ctx, "1", "empty")
	require.NoError(t, err)
	require.False(t, empty.HasMessages)
	require.Nil(t, empty.LastActive)
	_, _, err = r.GetThread(ctx, "2", "shared")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestDeployedSchemaConfigColumnPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		column any
		want   []entity.BizRefSummary
	}{
		{"explicit empty overrides legacy", "[]", []entity.BizRefSummary{}},
		{"column overrides legacy", `["current"]`, []entity.BizRefSummary{{Id: "current"}}},
		{"unset uses legacy", nil, []entity.BizRefSummary{{Id: "legacy"}}},
		{"null uses legacy", "null", []entity.BizRefSummary{{Id: "legacy"}}},
		{"malformed uses legacy", "invalid-json", []entity.BizRefSummary{{Id: "legacy"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := deployedSchemaReader(t)
			db := r.q.(*deployedSchemaQueryer).db
			_, err := db.ExecContext(context.Background(),
				`UPDATE agents SET config_json = ?, skill_package_ids = ? WHERE agent_id = 'a1'`,
				`{"model_ref":"stale","skill_package_ids":["legacy"],"max_iterations":99,"max_iters":8}`,
				tc.column)
			require.NoError(t, err)
			expert, _, err := r.GetExpert(context.Background(), "1", "a1")
			require.NoError(t, err)
			require.Equal(t, tc.want, expert.Config.SkillPackages)
			require.EqualValues(t, len(tc.want), *expert.SkillPkgCount)
			require.Equal(t, "provider/model-x", expert.Config.ModelRef)
			require.EqualValues(t, 8, *expert.Config.MaxIterations)
			require.Nil(t, expert.Config.TrajectoryOn, "missing setting must not get a guessed default")
		})
	}
}
