package bodyweight

import (
	"context"
	"fmt"
	"time"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

// TrendParams scopes a rolling-average trend computation. From and To are
// local-midnight times in Loc; both are inclusive.
type TrendParams struct {
	From       time.Time
	To         time.Time
	Loc        *time.Location
	WindowDays int
}

// TrendPoint is one date's entry in the trend response.
type TrendPoint struct {
	Date         string   `json:"date"`
	RollingAvgKg *float64 `json:"rolling_avg_kg"`
	SampleCount  int      `json:"sample_count"`
}

// Trend is the response shape for GET /weight/trend.
type Trend struct {
	From       string       `json:"from"`
	To         string       `json:"to"`
	TZ         string       `json:"tz"`
	WindowDays int          `json:"window_days"`
	Points     []TrendPoint `json:"points"`
}

// TrendFor returns the per-date rolling-average curve for [From, To] in p.Loc.
// Each point's average covers the trailing p.WindowDays days (inclusive of the
// current date). Dates with no samples in their window get null RollingAvgKg
// and SampleCount=0 — honest sparse-window signalling.
func (s *Service) TrendFor(ctx context.Context, p TrendParams) (*Trend, error) {
	if p.WindowDays < 1 {
		return nil, fmt.Errorf("window_days must be >= 1, got %d", p.WindowDays)
	}

	// Local midnight bounds for the date range.
	fromDay := time.Date(p.From.Year(), p.From.Month(), p.From.Day(), 0, 0, 0, 0, p.Loc)
	toDay := time.Date(p.To.Year(), p.To.Month(), p.To.Day(), 0, 0, 0, 0, p.Loc)

	// The earliest entry that could feed any window is (fromDay - (WindowDays-1)).
	windowStart := fromDay.AddDate(0, 0, -(p.WindowDays - 1))
	// Fetch through end-of-toDay; List uses half-open [from, to) so we add 24h.
	windowEnd := toDay.Add(24 * time.Hour)

	entries, err := s.repo.ListInRange(ctx, windowStart.UTC(), windowEnd.UTC())
	if err != nil {
		return nil, fmt.Errorf("list trend window: %w", err)
	}

	out := &Trend{
		From:       fromDay.Format("2006-01-02"),
		To:         toDay.Format("2006-01-02"),
		TZ:         p.Loc.String(),
		WindowDays: p.WindowDays,
		Points:     []TrendPoint{},
	}

	for d := fromDay; !d.After(toDay); d = d.AddDate(0, 0, 1) {
		// Trailing window: [d - (WindowDays-1) days, d + 24h) in local TZ → UTC.
		dayStart := d.AddDate(0, 0, -(p.WindowDays - 1))
		dayEnd := d.Add(24 * time.Hour)
		dayStartUTC := dayStart.UTC()
		dayEndUTC := dayEnd.UTC()

		var sum float64
		var count int
		for _, e := range entries {
			if !e.LoggedAt.Before(dayStartUTC) && e.LoggedAt.Before(dayEndUTC) {
				sum += e.WeightKg
				count++
			}
		}

		point := TrendPoint{
			Date:        d.Format("2006-01-02"),
			SampleCount: count,
		}
		if count > 0 {
			avg := numfmt.Round1(sum / float64(count))
			point.RollingAvgKg = &avg
		}
		out.Points = append(out.Points, point)
	}

	return out, nil
}
