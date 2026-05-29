package tplink

import (
	"net/url"
	"regexp"
	"strings"
)

var reScriptHelper = regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`)

func extractFirstScript(html string) string {
	m := reScriptHelper.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func formBody(data map[string]string) *strings.Reader {
	vals := url.Values{}
	for k, v := range data {
		vals.Set(k, v)
	}
	return strings.NewReader(vals.Encode())
}

func joinStrings(s []string) string {
	return strings.Join(s, ",")
}
