package sanitize

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type rule struct {
	Name    string
	Pattern string
	Replace string
}

var (
	credentialPatterns = []rule{
		{Name: "api_key", Pattern: `(?i)\bsk-[A-Za-z0-9_-]{8,}\b`, Replace: "[REDACTED]"},
		{Name: "bearer", Pattern: `(?i)\bBearer\s+[A-Za-z0-9._-]{12,}\b`, Replace: "[REDACTED]"},
		{Name: "aws_key", Pattern: `(?i)\bAKIA[0-9A-Z]{12,}\b`, Replace: "[REDACTED]"},
		{Name: "assignment", Pattern: `(?i)\b(password|passwd|api[_-]?key|secret|token|private[_-]?key)\s*[:=]\s*("[^"]*"|'[^']*'|[^\s,;}]+)`, Replace: "$1=[REDACTED]"},
	}
	policyPatterns = []rule{
		{Name: "instruction", Pattern: `(?i)\b(ignore|bypass|override)\b.{0,80}\b(system|developer|previous|prior|policy|instruction)s?\b`, Replace: "[FILTERED]"},
		{Name: "memory", Pattern: `(?i)\b(add|save|write|store|update|remember)\b.{0,80}\b(memory|forever|instruction|rule|policy)\b`, Replace: "[FILTERED]"},
		{Name: "workflow", Pattern: `(?i)\b(saved\s+(workflow\s+)?note\s+says|workflow objective|for this request, complete the following step)\b`, Replace: "[FILTERED]"},
		{Name: "jailbreak", Pattern: `(?i)\b(system prompt|developer message|hidden instruction|jailbreak|prompt injection|from now on)\b`, Replace: "[FILTERED]"},
	}
)

func init() {
	for i := range credentialPatterns {
		re := regexp.MustCompile(credentialPatterns[i].Pattern)
		credentialPatterns[i].Pattern = re.String()
	}
	for i := range policyPatterns {
		re := regexp.MustCompile(policyPatterns[i].Pattern)
		policyPatterns[i].Pattern = re.String()
	}
}

// Text sanitizes text by redacting credentials and filtering policy violations.
// Returns sanitized text and list of matched rule names.
func Text(text string) (string, []string) {
	if text == "" {
		return text, nil
	}

	removed := []string{}
	safe := text

	for _, r := range credentialPatterns {
		re := regexp.MustCompile(r.Pattern)
		if re.MatchString(safe) {
			removed = append(removed, r.Name)
			safe = re.ReplaceAllString(safe, r.Replace)
		}
	}

	for _, r := range policyPatterns {
		re := regexp.MustCompile(r.Pattern)
		if re.MatchString(safe) {
			removed = append(removed, r.Name)
			safe = re.ReplaceAllString(safe, r.Replace)
		}
	}

	return strings.TrimSpace(safe), uniqueStrings(removed)
}

// JSON recursively sanitizes a parsed JSON structure.
// Modifies the value in place, redacting sensitive keys and sanitizing strings.
// Returns list of removed paths.
func JSON(value *any, path string) []string {
	if value == nil {
		return nil
	}

	switch typed := (*value).(type) {
	case map[string]any:
		removed := []string{}
		for key, child := range typed {
			childPath := joinPath(path, key)
			if IsSensitiveKey(key) {
				typed[key] = "[REDACTED]"
				removed = append(removed, childPath)
				continue
			}
			removed = append(removed, JSON(&child, childPath)...)
			typed[key] = child
		}
		return removed
	case []any:
		removed := []string{}
		for idx, child := range typed {
			childPath := fmt.Sprintf("%s[%d]", path, idx)
			removed = append(removed, JSON(&child, childPath)...)
			typed[idx] = child
		}
		return removed
	case string:
		safe, matched := Text(typed)
		*value = safe
		for i := range matched {
			if path != "" {
				matched[i] = path + ":" + matched[i]
			}
		}
		return matched
	default:
		return nil
	}
}

// IsSensitiveKey checks if a JSON key name indicates sensitive data.
func IsSensitiveKey(key string) bool {
	normalized := strings.ToLower(key)
	for _, marker := range []string{"password", "passwd", "api_key", "apikey", "secret", "token", "private_key"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}