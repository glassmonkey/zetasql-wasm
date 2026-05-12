package ast

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

// RewriteNamedTimezoneLiterals rewrites TIMESTAMP literals whose
// string contents end in an IANA-named zone (e.g. "America/Los_Angeles")
// into the numeric-offset form the bundled WASM analyzer accepts.
// The wall time is preserved — only the trailing zone token changes
// shape — so the resolved instant the literal denotes is unchanged.
//
// This is one of the source-rewriting passes the package provides for
// adapting raw SQL to what the bundled C++ analyzer can resolve. The
// bundled WASM ships without tzdata (see
// wasm/assets/patches/zetasql_analyzer_options_timezone.patch), so
// named-zone TIMESTAMP literals would otherwise come back as "Invalid
// TIMESTAMP literal" before any caller can react. More such passes
// may surface as the WASM patch surface evolves; collect them at this
// level so callers do not have to know which one applies to a given
// input.
//
// root must be the parser AST corresponding to src — AST nodes carry
// parse-location ranges into src, and the rewrite operates on those
// absolute byte offsets. Returns src unchanged when there is nothing
// to rewrite. A named zone Go cannot resolve, or a datetime prefix
// none of the supported layouts can parse, is left alone so the
// downstream analyzer surfaces its native error.
func RewriteNamedTimezoneLiterals(root Node, src string) string {
	if root == nil {
		return src
	}
	var rewrites []sourceRewrite
	_ = Walk(root, func(n Node) error {
		lit, ok := n.(*DateOrTimeLiteralNode)
		if !ok {
			return nil
		}
		if lit.TypeKind() != generated.TypeKind_TYPE_TIMESTAMP {
			return nil
		}
		sl := lit.StringLiteral()
		if sl == nil {
			return nil
		}
		rebuilt, changed := rewriteNamedTimezoneInString(sl.StringValue())
		if !changed {
			return nil
		}
		loc, ok := ParseLocationOf(sl)
		if !ok {
			return nil
		}
		start := int(loc.Start)
		end := int(loc.End)
		if start < 0 || end > len(src) || start > end {
			return nil
		}
		replaced, ok := requoteStringLiteral(src[start:end], rebuilt)
		if !ok {
			return nil
		}
		rewrites = append(rewrites, sourceRewrite{loc.Start, loc.End, replaced})
		return nil
	})
	return applySourceRewrites(src, rewrites)
}

// sourceRewrite is one byte-range substitution applied during a
// source-rewriting pass. Kept unexported until a second pass surfaces
// a real need for callers to assemble batches themselves.
type sourceRewrite struct {
	start int32
	end   int32
	text  string
}

// applySourceRewrites returns src with each rewrite's [start,end)
// range replaced by text. Rewrites are applied right-to-left so
// earlier offsets remain valid; callers are responsible for keeping
// ranges disjoint.
func applySourceRewrites(src string, rewrites []sourceRewrite) string {
	if len(rewrites) == 0 {
		return src
	}
	sort.Slice(rewrites, func(i, j int) bool {
		return rewrites[i].start > rewrites[j].start
	})
	out := src
	for _, r := range rewrites {
		out = out[:r.start] + r.text + out[r.end:]
	}
	return out
}

// namedTZSuffixRE matches `<datetime> <Area/City[/...]>` at the end
// of a TIMESTAMP literal's string contents. The TZ token must contain
// at least one slash so plain words after the time (not valid TZ
// anyway) are not treated as zones.
var namedTZSuffixRE = regexp.MustCompile(
	`^(.*?\S)\s+([A-Za-z][A-Za-z0-9_+\-]*(?:/[A-Za-z][A-Za-z0-9_+\-]*)+)\s*$`,
)

// datetimeLayouts covers the BigQuery TIMESTAMP-literal datetime
// shapes documented at
// https://cloud.google.com/bigquery/docs/reference/standard-sql/lexical#timestamp_literals.
// The 9-digit fraction placeholder absorbs shorter fractions too.
var datetimeLayouts = []string{
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
}

func rewriteNamedTimezoneInString(s string) (string, bool) {
	m := namedTZSuffixRE.FindStringSubmatch(s)
	if m == nil {
		return s, false
	}
	datetimePart, tzName := m[1], m[2]
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return s, false
	}
	var parsed time.Time
	var parseErr error
	for _, layout := range datetimeLayouts {
		parsed, parseErr = time.ParseInLocation(layout, datetimePart, loc)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		return s, false
	}
	_, offsetSec := parsed.Zone()
	return datetimePart + formatTZOffset(offsetSec), true
}

func formatTZOffset(sec int) string {
	sign := "+"
	if sec < 0 {
		sign = "-"
		sec = -sec
	}
	hours := sec / 3600
	mins := (sec % 3600) / 60
	return fmt.Sprintf("%s%02d:%02d", sign, hours, mins)
}

// requoteStringLiteral wraps content in the same quote style as
// rawQuoted. rawQuoted is the full source span of a parsed
// StringLiteralNode and includes any leading prefix (e.g. `r`, `b`)
// plus the opening and closing quote runs (single, double, or
// triple). content must not contain the active quote char or
// backslashes; callers producing safe alphabets (digits, hyphens,
// colons, plus, period, space, T) get this for free.
func requoteStringLiteral(rawQuoted, content string) (string, bool) {
	i := 0
	for i < len(rawQuoted) && rawQuoted[i] != '\'' && rawQuoted[i] != '"' {
		i++
	}
	if i >= len(rawQuoted) {
		return "", false
	}
	q := rawQuoted[i]
	run := string(q)
	if strings.HasPrefix(rawQuoted[i:], string(q)+string(q)+string(q)) {
		run = strings.Repeat(string(q), 3)
	}
	return rawQuoted[:i] + run + content + run, true
}
