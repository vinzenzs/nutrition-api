package workouttemplates

import (
	"errors"
	"testing"
)

func ptrInt(i int) *int { return &i }

// validRunTemplate is the design-doc example: warmup, repeat ×5 of
// (interval @ power_zone, recovery @ hr_zone), cooldown.
func validRunTemplate() *Template {
	return &Template{
		Sport: SportRun,
		Name:  "VO2 intervals",
		Steps: []Step{
			{Type: NodeStep, Intent: IntentWarmup,
				Duration: &Duration{Kind: DurationTime, Seconds: ptrInt(600)},
				Target:   &Target{Kind: TargetHRZone, Low: ptrInt(1), High: ptrInt(2)}},
			{Type: NodeRepeat, Count: 5, Steps: []Step{
				{Type: NodeStep, Intent: IntentInterval,
					Duration: &Duration{Kind: DurationTime, Seconds: ptrInt(180)},
					Target:   &Target{Kind: TargetPowerZone, Low: ptrInt(4), High: ptrInt(4)}},
				{Type: NodeStep, Intent: IntentRecovery,
					Duration: &Duration{Kind: DurationTime, Seconds: ptrInt(120)},
					Target:   &Target{Kind: TargetHRZone, Low: ptrInt(1)}},
			}},
			{Type: NodeStep, Intent: IntentCooldown,
				Duration: &Duration{Kind: DurationTime, Seconds: ptrInt(300)},
				Target:   &Target{Kind: TargetHRZone, Low: ptrInt(1)}},
		},
	}
}

func TestValidate_AcceptsValidStructuredTemplate(t *testing.T) {
	if err := validateTemplate(validRunTemplate()); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestSumTimedDurationSec(t *testing.T) {
	// warmup 600 + 5×(180+120) + cooldown 300 = 2400s.
	if got := SumTimedDurationSec(validRunTemplate().Steps); got != 2400 {
		t.Fatalf("timed sum: want 2400, got %d", got)
	}

	// An all-untimed program (distance / open) contributes nothing.
	untimed := []Step{
		{Type: NodeStep, Intent: IntentActive,
			Duration: &Duration{Kind: DurationDistance, Meters: ptrInt(1000)},
			Target:   &Target{Kind: TargetNone}},
		{Type: NodeStep, Intent: IntentActive,
			Duration: &Duration{Kind: DurationOpen},
			Target:   &Target{Kind: TargetNone}},
	}
	if got := SumTimedDurationSec(untimed); got != 0 {
		t.Fatalf("untimed sum: want 0, got %d", got)
	}
}

func TestValidate_RejectsEmptySteps(t *testing.T) {
	tpl := validRunTemplate()
	tpl.Steps = nil
	assertErr(t, validateTemplate(tpl), ErrStepsEmpty)
}

func TestValidate_RejectsNestedRepeat(t *testing.T) {
	tpl := validRunTemplate()
	tpl.Steps = []Step{
		{Type: NodeRepeat, Count: 2, Steps: []Step{
			{Type: NodeRepeat, Count: 2, Steps: []Step{
				{Type: NodeStep, Intent: IntentActive, Duration: &Duration{Kind: DurationOpen}, Target: &Target{Kind: TargetNone}},
			}},
		}},
	}
	assertErr(t, validateTemplate(tpl), ErrRepeatNested)
}

func TestValidate_RejectsOutOfRangeZone(t *testing.T) {
	tpl := validRunTemplate()
	tpl.Steps = []Step{
		{Type: NodeStep, Intent: IntentInterval,
			Duration: &Duration{Kind: DurationTime, Seconds: ptrInt(60)},
			Target:   &Target{Kind: TargetHRZone, Low: ptrInt(1), High: ptrInt(6)}},
	}
	assertErr(t, validateTemplate(tpl), ErrTargetRangeInvalid)
}

func TestValidate_RejectsInvertedRange(t *testing.T) {
	tpl := validRunTemplate()
	tpl.Steps = []Step{
		{Type: NodeStep, Intent: IntentInterval,
			Duration: &Duration{Kind: DurationTime, Seconds: ptrInt(60)},
			Target:   &Target{Kind: TargetPace, LowSecPerKM: ptrInt(330), HighSecPerKM: ptrInt(300)}},
	}
	assertErr(t, validateTemplate(tpl), ErrTargetRangeInvalid)
}

func TestValidate_RejectsUnknownDurationKind(t *testing.T) {
	tpl := validRunTemplate()
	tpl.Steps = []Step{
		{Type: NodeStep, Intent: IntentActive,
			Duration: &Duration{Kind: "forever"}, Target: &Target{Kind: TargetNone}},
	}
	assertErr(t, validateTemplate(tpl), ErrDurationInvalid)
}

func TestValidate_RejectsUnknownTargetKind(t *testing.T) {
	tpl := validRunTemplate()
	tpl.Steps = []Step{
		{Type: NodeStep, Intent: IntentActive,
			Duration: &Duration{Kind: DurationOpen}, Target: &Target{Kind: "vibes"}},
	}
	assertErr(t, validateTemplate(tpl), ErrTargetInvalid)
}

func TestValidate_RejectsTimeDurationWithoutSeconds(t *testing.T) {
	tpl := validRunTemplate()
	tpl.Steps = []Step{
		{Type: NodeStep, Intent: IntentActive,
			Duration: &Duration{Kind: DurationTime}, Target: &Target{Kind: TargetNone}},
	}
	assertErr(t, validateTemplate(tpl), ErrDurationInvalid)
}

func TestValidate_RejectsRepeatCountBelowTwo(t *testing.T) {
	tpl := validRunTemplate()
	tpl.Steps = []Step{
		{Type: NodeRepeat, Count: 1, Steps: []Step{
			{Type: NodeStep, Intent: IntentActive, Duration: &Duration{Kind: DurationOpen}, Target: &Target{Kind: TargetNone}},
		}},
	}
	assertErr(t, validateTemplate(tpl), ErrRepeatInvalid)
}

func TestValidate_RejectsUnknownIntent(t *testing.T) {
	tpl := validRunTemplate()
	tpl.Steps = []Step{
		{Type: NodeStep, Intent: "sprint-ish", Duration: &Duration{Kind: DurationOpen}, Target: &Target{Kind: TargetNone}},
	}
	assertErr(t, validateTemplate(tpl), ErrIntentInvalid)
}

func TestValidate_RejectsUnknownNodeType(t *testing.T) {
	tpl := validRunTemplate()
	tpl.Steps = []Step{{Type: "superset"}}
	assertErr(t, validateTemplate(tpl), ErrStepTypeInvalid)
}

func TestValidate_RejectsBadSport(t *testing.T) {
	tpl := validRunTemplate()
	tpl.Sport = "kabaddi"
	assertErr(t, validateTemplate(tpl), ErrSportInvalid)
}

func TestValidate_RejectsEmptyName(t *testing.T) {
	tpl := validRunTemplate()
	tpl.Name = "   "
	assertErr(t, validateTemplate(tpl), ErrNameRequired)
}

func TestValidate_RejectsNonPositiveEstimated(t *testing.T) {
	tpl := validRunTemplate()
	tpl.EstimatedDurationSec = ptrInt(0)
	assertErr(t, validateTemplate(tpl), ErrEstimatedInvalid)
}

func assertErr(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
