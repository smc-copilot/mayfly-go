package langfuse

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	bearerSecret     = regexp.MustCompile(`(?i)(bearer\s+)[^\s"'<>]+`)
	assignedSecret   = regexp.MustCompile(`(?i)((?:api[_-]?key|access[_-]?token|refresh[_-]?token|password|secret|authorization|cookie)\s*[=:]\s*)[^\s,"'<>]+`)
	credentialHeader = regexp.MustCompile(`(?i)((?:proxy-authorization|authorization|set-cookie|cookie)\s*[:=]\s*)[^\r\n]+`)
)

func redactContent(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return json.RawMessage(`"[restricted]"`)
	}
	encoded, err := json.Marshal(redactValue(value))
	if err != nil {
		return json.RawMessage(`"[restricted]"`)
	}
	return encoded
}

func redactValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
			if strings.Contains(normalized, "password") || strings.Contains(normalized, "secret") || strings.Contains(normalized, "apikey") || normalized == "authorization" || normalized == "proxyauthorization" || normalized == "cookie" || normalized == "setcookie" || normalized == "token" || strings.Contains(normalized, "accesstoken") || strings.Contains(normalized, "refreshtoken") {
				v[key] = "[redacted]"
			} else {
				v[key] = redactValue(item)
			}
		}
		return v
	case []any:
		for i, item := range v {
			v[i] = redactValue(item)
		}
		return v
	case string:
		var nested any
		if json.Unmarshal([]byte(v), &nested) == nil {
			switch nested.(type) {
			case map[string]any, []any:
				return redactValue(nested)
			}
		}
		v = credentialHeader.ReplaceAllString(v, "${1}[redacted]")
		return assignedSecret.ReplaceAllString(bearerSecret.ReplaceAllString(v, "${1}[redacted]"), "${1}[redacted]")
	default:
		return value
	}
}
