package gates

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"html"
	"io"
	"mime"
	"regexp"
	"strings"
	"unicode/utf8"

	"aegisguard/internal/interfaces"
	"aegisguard/internal/sanitize"
)

var (
	htmlTagPattern         = regexp.MustCompile(`(?s)<[^>]*>`)
	htmlScriptStylePattern = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>`)
)

// ReturnGate 返回门控
type ReturnGate struct {
	policyEngine *PolicyEngine
}

// NewReturnGate 创建返回门控
func NewReturnGate() *ReturnGate {
	return NewReturnGateWithRuntime(nil)
}

func NewReturnGateWithRuntime(runtime *PolicyRuntime) *ReturnGate {
	return &ReturnGate{
		policyEngine: NewPolicyEngineWithRuntime(runtime),
	}
}

// Evaluate 评估返回结果
func (g *ReturnGate) Evaluate(body []byte) interfaces.EvaluateResult {
	return g.EvaluateWithMetadata(body, "", "")
}

// EvaluateWithMetadata evaluates a response after decoding supported content encodings
// and normalizing common non-JSON carriers such as HTML.
func (g *ReturnGate) EvaluateWithMetadata(body []byte, contentType, contentEncoding string) interfaces.EvaluateResult {
	normalizedBody, carrierRules, err := NormalizeReturnBody(body, contentType, contentEncoding)
	if err != nil {
		return makeEvaluateResult(Degrade, "return carrier could not be decoded and must be isolated", 55, []string{"return_carrier_unparseable"})
	}

	text := extractJSONText(normalizedBody, "content", "text", "message", "output", "tool_calls", "function_call")
	score, rules, ruleAction := g.policyEngine.ScoreForGate("return", text)
	rules = append(carrierRules, rules...)

	if hasRuleFromList(rules, "memory_poisoning") {
		return makeEvaluateResult(Block, "return contains memory or instruction contamination", score, rules)
	}
	if hasRuleFromList(rules, "illegal_finance") {
		return makeEvaluateResult(Deny, "return contains prohibited financial misconduct content", score, rules)
	}
	if ruleAction == Deny && g.policyEngine.ShouldHumanReview(score) {
		return makeEvaluateResult(Deny, "return denied by policy rule", score, rules)
	}
	if ruleAction == Block && g.policyEngine.ShouldHumanReview(score) {
		return makeEvaluateResult(Block, "return blocked by policy rule", score, rules)
	}
	if hasRuleFromList(rules, "sensitive_access") || hasRuleFromList(rules, "prompt_injection") {
		return makeEvaluateResult(Degrade, "return contains sensitive or contaminated content and must be sanitized", score, rules)
	}
	if g.policyEngine.ShouldBlock(score) {
		return makeEvaluateResult(Block, "return risk exceeds block threshold", score, rules)
	}
	if g.policyEngine.ShouldDegrade(score) {
		return makeEvaluateResult(Degrade, "return risk can be handled by sanitized summary", score, rules)
	}

	return makeEvaluateResult(Allow, "return passed policy checks", score, rules)
}

// Filter returns a safe degraded response body.
func (g *ReturnGate) Filter(body []byte) []byte {
	return g.FilterWithMetadata(body, "", "")
}

func (g *ReturnGate) FilterWithMetadata(body []byte, contentType, contentEncoding string) []byte {
	normalizedBody, _, err := NormalizeReturnBody(body, contentType, contentEncoding)
	if err != nil {
		return []byte(`{"error":"response body isolated because its carrier could not be decoded safely"}`)
	}

	var payload map[string]any
	if err := json.Unmarshal(normalizedBody, &payload); err != nil {
		safe, _ := sanitize.Text(string(normalizedBody))
		return []byte(safe)
	}

	var v any = payload
	sanitize.JSON(&v, "")
	filtered, err := json.Marshal(v)
	if err != nil {
		safe, _ := sanitize.Text(string(normalizedBody))
		return []byte(safe)
	}
	return filtered
}

func NormalizeReturnBody(body []byte, contentType, contentEncoding string) ([]byte, []string, error) {
	decoded, rules, err := DecodeReturnBody(body, contentEncoding)
	if err != nil {
		return nil, rules, err
	}

	mediaType := normalizeMediaType(contentType)
	if mediaType == "text/html" || looksLikeHTML(decoded) {
		text := extractHTMLText(decoded)
		rules = append(rules, "return_carrier_html")
		return []byte(text), uniquePolicyRules(rules), nil
	}
	if mediaType != "" && !isPolicyTextMediaType(mediaType) {
		if !json.Valid(decoded) && !looksLikeText(decoded) {
			rules = append(rules, "return_carrier_unstructured")
			return nil, uniquePolicyRules(rules), fmtUnparseableCarrierError(mediaType)
		}
	}
	return decoded, uniquePolicyRules(rules), nil
}

func DecodeReturnBody(body []byte, contentEncoding string) ([]byte, []string, error) {
	encoding := strings.ToLower(strings.TrimSpace(contentEncoding))
	switch encoding {
	case "", "identity":
		return body, nil, nil
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, []string{"return_carrier_gzip"}, err
		}
		defer reader.Close()
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return nil, []string{"return_carrier_gzip"}, err
		}
		return decoded, []string{"return_carrier_gzip"}, nil
	case "deflate":
		reader := flate.NewReader(bytes.NewReader(body))
		defer reader.Close()
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return nil, []string{"return_carrier_deflate"}, err
		}
		return decoded, []string{"return_carrier_deflate"}, nil
	default:
		return nil, []string{"return_carrier_unsupported_encoding"}, fmtUnparseableCarrierError(encoding)
	}
}

func normalizeMediaType(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	}
	return strings.ToLower(mediaType)
}

func isPolicyTextMediaType(mediaType string) bool {
	if mediaType == "" {
		return true
	}
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	switch mediaType {
	case "application/json", "application/xml", "application/javascript", "application/x-ndjson":
		return true
	default:
		return strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml")
	}
}

func extractHTMLText(body []byte) string {
	text := string(body)
	text = htmlScriptStylePattern.ReplaceAllString(text, " ")
	text = htmlTagPattern.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	text = strings.Join(strings.Fields(text), " ")
	return text
}

func looksLikeHTML(body []byte) bool {
	text := strings.TrimSpace(strings.ToLower(string(body)))
	return strings.HasPrefix(text, "<!doctype html") || strings.HasPrefix(text, "<html") || strings.Contains(text, "<body")
}

func looksLikeText(body []byte) bool {
	if len(body) == 0 {
		return true
	}
	if !utf8.Valid(body) {
		return false
	}
	printable := 0
	for _, r := range string(body) {
		if r == '\n' || r == '\r' || r == '\t' || r >= 0x20 {
			printable++
		}
	}
	return printable*100/len([]rune(string(body))) >= 90
}

func uniquePolicyRules(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func fmtUnparseableCarrierError(carrier string) error {
	return &returnCarrierError{carrier: carrier}
}

type returnCarrierError struct {
	carrier string
}

func (e *returnCarrierError) Error() string {
	return "unsupported or unparseable return carrier: " + e.carrier
}
