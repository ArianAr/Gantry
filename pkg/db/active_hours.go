package db

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// hourWindow is an inclusive-start exclusive-end window in minutes from UTC midnight.
// Overnight windows (e.g. 22:00-06:00) have end < start.
type hourWindow struct {
	startMin int
	endMin   int
}

// ParseActiveHours parses "HH:MM-HH:MM" entries separated by commas.
// Empty string is valid (always active). Returns an error on malformed input.
func ParseActiveHours(spec string) ([]hourWindow, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	parts := strings.Split(spec, ",")
	out := make([]hourWindow, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		segs := strings.Split(p, "-")
		if len(segs) != 2 {
			return nil, fmt.Errorf("active hours entry %q: want HH:MM-HH:MM", p)
		}
		start, err := parseHHMM(strings.TrimSpace(segs[0]))
		if err != nil {
			return nil, fmt.Errorf("active hours start in %q: %w", p, err)
		}
		end, err := parseHHMM(strings.TrimSpace(segs[1]))
		if err != nil {
			return nil, fmt.Errorf("active hours end in %q: %w", p, err)
		}
		if start == end {
			return nil, fmt.Errorf("active hours entry %q: start and end must differ", p)
		}
		out = append(out, hourWindow{startMin: start, endMin: end})
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// ValidateActiveHours returns an error if the spec is malformed.
func ValidateActiveHours(spec string) error {
	_, err := ParseActiveHours(spec)
	return err
}

// InActiveHours reports whether now (any location; converted to UTC) falls in the windows.
// Empty/malformed empty = always true; malformed non-empty should be validated before use.
func InActiveHours(spec string, now time.Time) bool {
	windows, err := ParseActiveHours(spec)
	if err != nil || len(windows) == 0 {
		// Empty => always; invalid treated as always-on at runtime (StartJob validates first).
		return err == nil
	}
	mins := now.UTC().Hour()*60 + now.UTC().Minute()
	for _, w := range windows {
		if w.contains(mins) {
			return true
		}
	}
	return false
}

func (w hourWindow) contains(mins int) bool {
	if w.startMin < w.endMin {
		return mins >= w.startMin && mins < w.endMin
	}
	// overnight: e.g. 22:00-06:00
	return mins >= w.startMin || mins < w.endMin
}

func parseHHMM(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("want HH:MM, got %q", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("invalid hour in %q", s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid minute in %q", s)
	}
	return h*60 + m, nil
}

// ReverseRule builds a one-shot reverse of r for bidirectional sync.
// Reverse never deletes, never fans out, and is not itself bidirectional.
func ReverseRule(r *SyncRule) *SyncRule {
	if r == nil {
		return nil
	}
	rev := *r
	rev.SourceProviderID = r.TargetProviderID
	rev.SourceBucket = r.TargetBucket
	rev.SourcePrefix = r.TargetPrefix
	rev.TargetProviderID = r.SourceProviderID
	rev.TargetBucket = r.SourceBucket
	rev.TargetPrefix = r.SourcePrefix
	rev.DeleteOnTarget = false
	rev.ExtraTargets = ""
	rev.Bidirectional = false
	rev.Name = r.Name + " (reverse)"
	return &rev
}
