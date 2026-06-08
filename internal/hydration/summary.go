package hydration

import (
	"context"
	"fmt"
	"time"
)

// DailyParams scopes a daily hydration summary request.
type DailyParams struct {
	Date time.Time
	Loc  *time.Location
}

// Daily is the response shape for GET /summary/hydration/daily.
type Daily struct {
	Date       string   `json:"date"`
	TZ         string   `json:"tz"`
	TotalMl    float64  `json:"total_ml"`
	EntryCount int      `json:"entry_count"`
	Entries    []*Entry `json:"entries"`
}

// DailyFor computes the daily hydration summary for p.Date in p.Loc.
func (s *Service) DailyFor(ctx context.Context, p DailyParams) (*Daily, error) {
	dayStart := time.Date(p.Date.Year(), p.Date.Month(), p.Date.Day(), 0, 0, 0, 0, p.Loc)
	dayEnd := dayStart.Add(24 * time.Hour)

	entries, err := s.repo.List(ctx, dayStart.UTC(), dayEnd.UTC())
	if err != nil {
		return nil, fmt.Errorf("list daily hydration: %w", err)
	}
	if entries == nil {
		entries = []*Entry{}
	}
	var total float64
	for _, e := range entries {
		total += e.QuantityMl
	}
	return &Daily{
		Date:       p.Date.Format("2006-01-02"),
		TZ:         p.Loc.String(),
		TotalMl:    total,
		EntryCount: len(entries),
		Entries:    entries,
	}, nil
}
