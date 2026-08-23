package util

import (
	"path"
	"strings"
)

// CleanHref canonicalizes a href for lookup: strips fragment/query and cleans the path.
func CleanHref(href string) string {
	if i := strings.IndexAny(href, "#?"); i != -1 {
		href = href[:i]
	}
	if href == "" {
		return "."
	}
	return path.Clean(href)
}
