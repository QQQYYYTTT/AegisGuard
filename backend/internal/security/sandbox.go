package security

import "regexp"

type SandboxResult struct {
	FingerprintSM3 string `json:"fingerprint_sm3"`
	SourceTag      string `json:"source_tag"`
	BlockedMarkers int    `json:"blocked_markers"`
	TrustedSummary string `json:"trusted_summary"`
}

func RunMemorySandbox(rawText string) SandboxResult {
	markers := []*regexp.Regexp{
		regexp.MustCompile(`(?i)ignore all safety rules`),
		regexp.MustCompile(`(?i)ignore previous instructions`),
		regexp.MustCompile(`(?i)remember this command forever`),
		regexp.MustCompile(`写入长期记忆`),
		regexp.MustCompile(`忽略规则`),
	}

	cleaned := rawText
	hitCount := 0
	for _, pattern := range markers {
		if pattern.MatchString(cleaned) {
			hitCount++
			cleaned = pattern.ReplaceAllString(cleaned, "[filtered]")
		}
	}

	if len(cleaned) > 240 {
		cleaned = cleaned[:240] + "..."
	}

	return SandboxResult{
		FingerprintSM3: SimpleHash(rawText),
		SourceTag:      "untrusted_external_result",
		BlockedMarkers: hitCount,
		TrustedSummary: cleaned,
	}
}
