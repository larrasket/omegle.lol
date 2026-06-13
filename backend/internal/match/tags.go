package match

import (
	"errors"
	"strings"
)

var (
	ErrTooManyTags = errors.New("too_many_tags")
	ErrTagTooLong  = errors.New("tag_too_long")
	ErrInvalidTag  = errors.New("invalid_tag")
)

// NormalizeTags applies the rules from spec §5.1:
// trim, lowercase, dedupe, charset [a-z0-9 -], drop empties, enforce limits.
func NormalizeTags(in []string, maxTags, maxLen int) ([]string, error) {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		t := strings.ToLower(strings.TrimSpace(raw))
		if t == "" {
			continue
		}
		if len(t) > maxLen {
			return nil, ErrTagTooLong
		}
		if !validTag(t) {
			return nil, ErrInvalidTag
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	if len(out) > maxTags {
		return nil, ErrTooManyTags
	}
	return out, nil
}

func validTag(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == ' ' || r == '-':
		default:
			return false
		}
	}
	return true
}
