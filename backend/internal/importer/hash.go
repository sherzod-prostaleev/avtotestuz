package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// ContentHash is a stable digest of everything that defines a question's
// content: category, image ref, per-locale texts, answers (position, text,
// correctness) and linked signs. Used for change detection and dedupe.
func ContentHash(q CanonQuestion) string {
	var b strings.Builder
	b.WriteString("cat:" + q.Category + "\n")
	b.WriteString("img:" + q.Image + "\n")
	writeLocaleMap(&b, "qt", q.Texts)
	answers := append([]CanonAnswer(nil), q.Answers...)
	sort.Slice(answers, func(i, j int) bool { return answers[i].Position < answers[j].Position })
	for _, a := range answers {
		fmt.Fprintf(&b, "a%d:%v:%s\n", a.Position, a.Correct, a.Image)
		writeLocaleMap(&b, fmt.Sprintf("a%dt", a.Position), a.Texts)
	}
	signs := append([]string(nil), q.Signs...)
	sort.Strings(signs)
	b.WriteString("signs:" + strings.Join(signs, ",") + "\n")
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func writeLocaleMap(b *strings.Builder, prefix string, m map[string]string) {
	locales := make([]string, 0, len(m))
	for l := range m {
		locales = append(locales, l)
	}
	sort.Strings(locales)
	for _, l := range locales {
		b.WriteString(prefix + ":" + l + ":" + m[l] + "\n")
	}
}
