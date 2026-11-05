package redaction

import (
	"encoding/json"
	"strings"
)

// 集中库允许保留的元数据白名单。聊天全文、prompt、工具参数/结果、密码、令牌一律丢弃。
var blockedKeys = map[string]struct{}{
	"payload": {}, "payload_json": {}, "payloadjson": {},
	"content": {}, "message": {}, "messages": {}, "body": {},
	"prompt": {}, "system_prompt": {}, "systemprompt": {}, "system": {},
	"args": {}, "arguments": {}, "input": {}, "inputs": {},
	"result": {}, "results": {}, "output": {}, "outputs": {},
	"password": {}, "passwd": {}, "secret": {}, "token": {}, "access_token": {},
	"refresh_token": {}, "authorization": {}, "cookie": {},
	"email_body": {}, "mail_body": {}, "document": {}, "file_content": {},
	"chat": {}, "transcript": {}, "text": {},
}

var allowedKeys = map[string]struct{}{
	"id": {}, "event_id": {}, "eventid": {},
	"thread_id": {}, "threadid": {}, "turn_id": {}, "turnid": {},
	"user_id": {}, "userid": {}, "username": {}, "user_name": {},
	"agent_id": {}, "agentid": {}, "agent_name": {}, "agentname": {},
	"action": {}, "event_type": {}, "eventtype": {}, "step_type": {}, "steptype": {},
	"tool_name": {}, "toolname": {}, "status": {}, "success": {}, "result_code": {},
	"target": {}, "model": {}, "source": {},
	"input_tokens": {}, "inputtokens": {}, "output_tokens": {}, "outputtokens": {},
	"duration_ms": {}, "durationms": {}, "duration": {}, "latency_ms": {},
	"created_at": {}, "createdat": {}, "updated_at": {}, "timestamp": {}, "ts": {},
}

const Uncollected = "未采集"

// Extract 从源行按白名单提取可入库字段。payload_json 等即使出现也只解析其中白名单键。
func Extract(row map[string]any) map[string]any {
	out := make(map[string]any)
	if row == nil {
		return out
	}
	for k, v := range row {
		nk := normalize(k)
		if _, blocked := blockedKeys[nk]; blocked {
			continue
		}
		if _, ok := allowedKeys[nk]; ok {
			out[nk] = v
		}
	}
	if raw, ok := row["payload_json"]; ok {
		mergePayloadWhitelist(out, raw)
	}
	if raw, ok := row["payload"]; ok {
		mergePayloadWhitelist(out, raw)
	}
	return out
}

func mergePayloadWhitelist(out map[string]any, raw any) {
	obj, ok := asObject(raw)
	if !ok {
		return
	}
	for k, v := range obj {
		nk := normalize(k)
		if _, blocked := blockedKeys[nk]; blocked {
			continue
		}
		if _, ok := allowedKeys[nk]; !ok {
			continue
		}
		if _, exists := out[nk]; !exists {
			out[nk] = v
		}
	}
}

func asObject(raw any) (map[string]any, bool) {
	switch v := raw.(type) {
	case map[string]any:
		return v, true
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, false
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(v), &obj); err != nil {
			return nil, false
		}
		return obj, true
	case []byte:
		var obj map[string]any
		if err := json.Unmarshal(v, &obj); err != nil {
			return nil, false
		}
		return obj, true
	default:
		return nil, false
	}
}

func normalize(k string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(k), "-", "_"))
}

func String(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[normalize(k)]; ok && v != nil {
			s := strings.TrimSpace(toString(v))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func Int64(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[normalize(k)]; ok && v != nil {
			switch n := v.(type) {
			case int64:
				return n
			case int:
				return int64(n)
			case float64:
				return int64(n)
			case json.Number:
				i, err := n.Int64()
				if err == nil {
					return i
				}
			case string:
				if n == "" {
					continue
				}
				var f float64
				if err := json.Unmarshal([]byte(n), &f); err == nil {
					return int64(f)
				}
			}
		}
	}
	return 0
}

func toString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		b, err := json.Marshal(s)
		if err != nil {
			return ""
		}
		return strings.Trim(string(b), `"`)
	}
}
