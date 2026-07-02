package gates

import (
	"encoding/base64"
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var (
	zeroWidthReplacer = strings.NewReplacer(
		"\u200b", "",
		"\u200c", "",
		"\u200d", "",
		"\ufeff", "",
		"\u2060", "",
	)
	whitespacePattern    = regexp.MustCompile(`\s+`)
	base64TokenPattern   = regexp.MustCompile(`(?i)\b[A-Za-z0-9+/_-]{12,}={0,2}\b`)
	maxNormalizationPass = 8
)

// NormalizeForPolicy 对待检测文本执行轻量归一化与有限解码。
// 目标不是“理解”语义，而是把常见的编码/混淆手法还原成规则引擎可匹配的形态。
func NormalizeForPolicy(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	queue := []string{text}
	seen := make(map[string]struct{}, maxNormalizationPass)
	var variants []string

	for len(queue) > 0 && len(variants) < maxNormalizationPass {
		current := queue[0]
		queue = queue[1:]

		canonical := canonicalizePolicyText(current)
		if canonical == "" {
			continue
		}
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		variants = append(variants, canonical)

		if decoded := canonicalizePolicyText(html.UnescapeString(canonical)); decoded != "" && decoded != canonical {
			queue = append(queue, decoded)
		}
		if decoded := decodeURLRecursively(canonical, 2); decoded != "" && decoded != canonical {
			queue = append(queue, decoded)
		}
		for _, decoded := range decodeEmbeddedBase64(canonical) {
			if decoded != canonical {
				queue = append(queue, decoded)
			}
		}
	}

	return strings.Join(variants, "\n")
}

func canonicalizePolicyText(text string) string {
	if text == "" {
		return ""
	}
	text = strings.ToValidUTF8(text, " ")
	text = norm.NFKC.String(text)
	text = zeroWidthReplacer.Replace(text)

	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		case unicode.IsControl(r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}

	return strings.TrimSpace(whitespacePattern.ReplaceAllString(b.String(), " "))
}

func decodeURLRecursively(text string, rounds int) string {
	decoded := text
	for i := 0; i < rounds; i++ {
		next, err := url.QueryUnescape(decoded)
		if err != nil || next == decoded {
			break
		}
		decoded = next
	}
	return canonicalizePolicyText(decoded)
}

func decodeEmbeddedBase64(text string) []string {
	matches := base64TokenPattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}

	results := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, token := range matches {
		for _, encoding := range []*base64.Encoding{
			base64.StdEncoding,
			base64.RawStdEncoding,
			base64.URLEncoding,
			base64.RawURLEncoding,
		} {
			decoded, err := encoding.DecodeString(token)
			if err != nil || !looksLikeReadableText(decoded) {
				continue
			}
			canonical := canonicalizePolicyText(string(decoded))
			if canonical == "" {
				continue
			}
			if _, ok := seen[canonical]; ok {
				continue
			}
			seen[canonical] = struct{}{}
			results = append(results, canonical)
		}
	}
	return results
}

func looksLikeReadableText(data []byte) bool {
	if len(data) < 6 || len(data) > 4096 || !utf8.Valid(data) {
		return false
	}

	printable := 0
	letters := 0
	for _, r := range string(data) {
		switch {
		case unicode.IsLetter(r):
			letters++
			printable++
		case unicode.IsDigit(r), unicode.IsSpace(r), unicode.IsPunct(r), unicode.IsSymbol(r):
			printable++
		}
	}

	if letters == 0 {
		return false
	}
	return printable*100/len([]rune(string(data))) >= 85
}
