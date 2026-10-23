package util

import "strings"

// SplitSubjects splits each value on semicolons, trims whitespace from every
// part, and drops empty parts. Source order and duplicates are preserved.
func SplitSubjects(values []string) []string {
	var subjects []string
	for _, value := range values {
		for _, part := range strings.Split(value, ";") {
			if part = strings.TrimSpace(part); part != "" {
				subjects = append(subjects, part)
			}
		}
	}
	return subjects
}

// DedupeSubjects drops later duplicate values case-insensitively, preserving
// the first spelling and position. It returns nil for an empty input.
func DedupeSubjects(subjects []string) []string {
	if len(subjects) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(subjects))
	out := subjects[:0]
	for _, subject := range subjects {
		key := strings.ToLower(subject)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, subject)
	}
	return out
}
