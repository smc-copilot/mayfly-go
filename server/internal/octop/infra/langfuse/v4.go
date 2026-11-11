package langfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mayfly-go/internal/octop/domain/entity"
)

type rawV4Observation struct {
	rawObservation
	ProjectId    string           `json:"projectId"`
	SessionId    string           `json:"sessionId"`
	UserId       string           `json:"userId"`
	TraceName    string           `json:"traceName"`
	Tags         []string         `json:"tags"`
	UsageDetails map[string]int64 `json:"usageDetails"`
}

type v4Page struct {
	Data []rawV4Observation `json:"data"`
	Meta *struct {
		Cursor string `json:"cursor"`
	} `json:"meta"`
}

func boundedWindow(from, to *time.Time) (time.Time, time.Time, error) {
	end := time.Now().UTC()
	if to != nil {
		end = to.UTC()
	}
	start := end.AddDate(0, -1, 0)
	if from != nil {
		start = from.UTC()
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, ErrQuery
	}
	return start, end, nil
}

func (c *Client) queryV4(ctx context.Context, project string, q ListObservationsQuery, observationId string, roots, content bool) (*v4Page, error) {
	if q.SessionId == "" || q.UserId == "" || (q.TraceId == "" && !roots) || len(q.Cursor) > 8192 {
		return nil, ErrQuery
	}
	from, to, err := boundedWindow(q.From, q.To)
	if err != nil {
		return nil, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	fields := "core,basic,trace_context,model,usage"
	if content {
		fields = "core,basic,io"
	}
	params := url.Values{"fields": {fields}, "limit": {strconv.Itoa(limit)}, "sessionId": {q.SessionId}, "userId": {q.UserId}, "fromStartTime": {from.Format(time.RFC3339Nano)}, "toStartTime": {to.Format(time.RFC3339Nano)}}
	if q.TraceId != "" {
		params.Set("traceId", q.TraceId)
	}
	if q.Cursor != "" {
		params.Set("cursor", q.Cursor)
	}
	if roots {
		params.Set("isRootObservation", "true")
	}
	// Advanced filters take precedence over the ordinary scope parameters.
	// Include the entire scope when selecting content by observation ID.
	if observationId != "" {
		conditions := []map[string]any{
			{"type": "string", "column": "id", "operator": "=", "value": observationId},
			{"type": "string", "column": "traceId", "operator": "=", "value": q.TraceId},
			{"type": "string", "column": "sessionId", "operator": "=", "value": q.SessionId},
			{"type": "string", "column": "userId", "operator": "=", "value": q.UserId},
			{"type": "datetime", "column": "startTime", "operator": ">=", "value": from.Format(time.RFC3339Nano)},
			{"type": "datetime", "column": "startTime", "operator": "<", "value": to.Format(time.RFC3339Nano)},
		}
		encoded, err := json.Marshal(conditions)
		if err != nil {
			return nil, err
		}
		params.Set("filter", string(encoded))
	}
	body, status, err := c.do(ctx, http.MethodGet, "/api/public/v2/observations?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	if err := mapStatus(status); err != nil {
		return nil, err
	}
	var result v4Page
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("%w: observations response", ErrUnsupported)
	}
	if result.Data == nil || result.Meta == nil {
		return nil, fmt.Errorf("%w: observations envelope", ErrUnsupported)
	}
	if result.Meta.Cursor != "" && result.Meta.Cursor == q.Cursor {
		return nil, fmt.Errorf("%w: non-progressing cursor", ErrUnsupported)
	}
	for _, row := range result.Data {
		if row.ID == "" || row.TraceID == "" || row.ProjectId != project || row.SessionId != q.SessionId || row.UserId != q.UserId || row.StartTime == nil {
			return nil, ErrAssociation
		}
		if row.StartTime.Before(from) || !row.StartTime.Before(to) || (q.TraceId != "" && row.TraceID != q.TraceId) || (observationId != "" && row.ID != observationId) {
			return nil, ErrAssociation
		}
	}
	return &result, nil
}

func (c *Client) listV4Traces(ctx context.Context, q ListTracesQuery, project string) (*ListTracesResult, error) {
	page, err := c.queryV4(ctx, project, ListObservationsQuery{SessionId: q.SessionId, UserId: q.UserId, TraceId: q.TraceId, From: q.From, To: q.To, Cursor: q.Cursor, Limit: q.Limit}, "", true, false)
	if err != nil {
		return nil, err
	}
	items := make([]*entity.BizTrace, 0, len(page.Data))
	byId := make(map[string]*entity.BizTrace, len(page.Data))
	for _, row := range page.Data {
		if found := byId[row.TraceID]; found != nil {
			if row.StartTime.Before(*found.StartTime) {
				found.StartTime = row.StartTime
			}
			if row.EndTime != nil && (found.EndTime == nil || row.EndTime.After(*found.EndTime)) {
				found.EndTime = row.EndTime
			}
			found.HasError = found.HasError || strings.EqualFold(row.Level, "ERROR") || strings.EqualFold(row.Level, "WARNING")
			continue
		}
		name := row.TraceName
		if name == "" {
			name = row.Name
		}
		item := &entity.BizTrace{TraceId: row.TraceID, Name: name, SessionId: row.SessionId, UserId: row.UserId, StartTime: row.StartTime, EndTime: row.EndTime, Level: row.Level, Tags: row.Tags, HasError: strings.EqualFold(row.Level, "ERROR") || strings.EqualFold(row.Level, "WARNING")}
		byId[item.TraceId] = item
		items = append(items, item)
	}
	for _, item := range items {
		if item.EndTime != nil {
			ms := item.EndTime.Sub(*item.StartTime).Milliseconds()
			if ms >= 0 {
				item.LatencyMs = &ms
			}
		}
	}
	return &ListTracesResult{Items: items, Total: -1, Page: q.Page, Limit: q.Limit, HasMore: page.Meta.Cursor != "", NextCursor: page.Meta.Cursor}, nil
}

func (c *Client) listV4Observations(ctx context.Context, q ListObservationsQuery, project string) (*ListObservationsResult, error) {
	page, err := c.queryV4(ctx, project, q, "", false, false)
	if err != nil {
		return nil, err
	}
	items := make([]*entity.BizObservation, 0, len(page.Data))
	seen := make(map[string]bool, len(page.Data))
	for _, row := range page.Data {
		if seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		item := row.toBiz()
		if n, ok := row.UsageDetails["input"]; ok {
			item.InputTokens = &n
		}
		if n, ok := row.UsageDetails["output"]; ok {
			item.OutputTokens = &n
		}
		items = append(items, item)
	}
	return &ListObservationsResult{Items: items, Total: -1, HasMore: page.Meta.Cursor != "", NextCursor: page.Meta.Cursor}, nil
}

// GetObservationInScope never reads an arbitrary v4 ID without the authenticated
// project, source user, session, trace and time boundaries.
func (c *Client) GetObservationInScope(ctx context.Context, id string, q ListObservationsQuery) (*entity.BizObservation, *entity.BizObservationContent, error) {
	info, err := c.Verify(ctx)
	if err != nil {
		return nil, nil, err
	}
	if info.APIMode == ModeClassic {
		obs, content, err := c.getClassicObservation(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		from, to, err := boundedWindow(q.From, q.To)
		if err != nil {
			return nil, nil, err
		}
		if obs.ObservationId != id || obs.TraceId != q.TraceId || obs.StartTime == nil || obs.StartTime.Before(from) || !obs.StartTime.Before(to) {
			return nil, nil, ErrAssociation
		}
		return obs, content, nil
	}
	if id == "" {
		return nil, nil, ErrQuery
	}
	q.Limit = 2
	page, err := c.queryV4(ctx, info.ProjectId, q, id, false, true)
	if err != nil {
		return nil, nil, err
	}
	if len(page.Data) != 1 || page.Meta.Cursor != "" {
		return nil, nil, ErrAssociation
	}
	row := page.Data[0]
	obs := row.toBiz()
	input, inTruncated := stringifyTruncate(redactContent(row.Input), MaxContentFieldBytes)
	output, outTruncated := stringifyTruncate(redactContent(row.Output), MaxContentFieldBytes)
	return obs, &entity.BizObservationContent{ObservationId: obs.ObservationId, Input: input, Output: output, Truncated: inTruncated || outTruncated}, nil
}
