package book

import (
	"path"
	"strings"
)

// cleanHref canonicalizes a href for resource lookup: strips fragment/query and cleans the path.
func cleanHref(href string) string {
	if i := strings.IndexAny(href, "#?"); i != -1 {
		href = href[:i]
	}
	if href == "" {
		return "."
	}
	return path.Clean(href)
}
