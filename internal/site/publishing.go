package site

import (
	"fmt"
	"strings"
	"time"
)

// ReadOptions controls publication selection consistently across every command.
// A zero AsOf samples the wall clock once per read; each override is independent.
type ReadOptions struct {
	Drafts  bool
	Future  bool
	Expired bool
	AsOf    time.Time
}

// Visible uses [publish_date, expiry_date). The editorial Date never schedules
// content, preserving v0.2 behavior. Missing bounds are unrestricted.
func (p Page) Visible(options ReadOptions, now time.Time) bool {
	return (!p.Draft || options.Drafts) &&
		(p.PublishDate.IsZero() || !now.Before(p.PublishDate) || options.Future) &&
		(p.ExpiryDate.IsZero() || now.Before(p.ExpiryDate) || options.Expired)
}

func publicationTime(value any) (time.Time, error) {
	if value == nil {
		return time.Time{}, nil
	}
	var result time.Time
	switch v := value.(type) {
	case string:
		var err error
		result, err = time.Parse("2006-01-02", v)
		if err != nil {
			result, err = time.Parse(time.RFC3339Nano, v)
		}
		if err != nil {
			return time.Time{}, fmt.Errorf("expected YYYY-MM-DD or RFC3339 timestamp with timezone")
		}
	case time.Time:
		switch v.Location().String() {
		case "date-local":
			result = time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
		case "datetime-local", "time-local":
			return time.Time{}, fmt.Errorf("timestamp requires an explicit timezone")
		default:
			result = v
		}
	default:
		return time.Time{}, fmt.Errorf("expected a date or timestamp, got %T", value)
	}
	if result.IsZero() {
		return time.Time{}, fmt.Errorf("zero timestamp is reserved for an absent boundary")
	}
	return result.UTC(), nil
}

// Terms are display labels for now; taxonomy routes are a later milestone.
func terms(source, field string, values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("%s: %s cannot contain an empty label", source, field)
		}
		if !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	return result, nil
}
