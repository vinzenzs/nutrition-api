package trainingplan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wt "github.com/vinzenzs/kazper/internal/workouttemplates"
)

func intp(i int) *int { return &i }

func paceTarget(lo, hi int) wt.Target {
	return wt.Target{Kind: wt.TargetPace, LowSecPerKM: intp(lo), HighSecPerKM: intp(hi)}
}

func TestValidateOverrides(t *testing.T) {
	t.Run("valid pace override accepted", func(t *testing.T) {
		require.NoError(t, validateOverrides([]SlotTargetOverride{{Intent: wt.IntentActive, Target: paceTarget(435, 435)}}))
	})
	t.Run("unknown intent rejected", func(t *testing.T) {
		assert.ErrorIs(t, validateOverrides([]SlotTargetOverride{{Intent: "sprint", Target: paceTarget(300, 320)}}), ErrOverrideIntentInvalid)
	})
	t.Run("duplicate intent rejected", func(t *testing.T) {
		err := validateOverrides([]SlotTargetOverride{
			{Intent: wt.IntentInterval, Target: paceTarget(300, 320)},
			{Intent: wt.IntentInterval, Target: paceTarget(310, 330)},
		})
		assert.ErrorIs(t, err, ErrOverrideDuplicate)
	})
	t.Run("inverted pace range rejected", func(t *testing.T) {
		assert.ErrorIs(t, validateOverrides([]SlotTargetOverride{{Intent: wt.IntentActive, Target: paceTarget(440, 420)}}), ErrOverrideTargetInvalid)
	})
	t.Run("out-of-range zone rejected", func(t *testing.T) {
		bad := wt.Target{Kind: wt.TargetHRZone, Low: intp(0), High: intp(9)}
		assert.ErrorIs(t, validateOverrides([]SlotTargetOverride{{Intent: wt.IntentActive, Target: bad}}), ErrOverrideTargetInvalid)
	})
	t.Run("unknown target kind rejected", func(t *testing.T) {
		assert.ErrorIs(t, validateOverrides([]SlotTargetOverride{{Intent: wt.IntentActive, Target: wt.Target{Kind: "bananas"}}}), ErrOverrideTargetInvalid)
	})
	t.Run("empty list ok", func(t *testing.T) {
		require.NoError(t, validateOverrides(nil))
	})
}

func TestApplyOverrides_ReplacesMatchingIntentInsideRepeat(t *testing.T) {
	steps := []wt.Step{
		{Type: wt.NodeStep, Intent: wt.IntentWarmup, Target: &wt.Target{Kind: wt.TargetHRZone, Low: intp(1), High: intp(2)}},
		{Type: wt.NodeRepeat, Count: 5, Steps: []wt.Step{
			{Type: wt.NodeStep, Intent: wt.IntentInterval, Target: &wt.Target{Kind: wt.TargetPowerZone, Low: intp(4), High: intp(4)}},
			{Type: wt.NodeStep, Intent: wt.IntentRecovery, Target: &wt.Target{Kind: wt.TargetHRZone, Low: intp(1), High: intp(1)}},
		}},
	}
	pace := paceTarget(435, 435)
	out := applyOverrides(steps, []SlotTargetOverride{{Intent: wt.IntentInterval, Target: pace}}, nil)

	// Interval step (inside the repeat) is now pace.
	interval := out[1].Steps[0]
	assert.Equal(t, wt.TargetPace, interval.Target.Kind)
	assert.Equal(t, 435, *interval.Target.LowSecPerKM)
	// Recovery and warmup are unchanged.
	assert.Equal(t, wt.TargetHRZone, out[1].Steps[1].Target.Kind)
	assert.Equal(t, wt.TargetHRZone, out[0].Target.Kind)
	// Repeat structure preserved.
	assert.Equal(t, 5, out[1].Count)
	// The original template slice was not mutated.
	assert.Equal(t, wt.TargetPowerZone, steps[1].Steps[0].Target.Kind)
}

func TestApplyOverrides_NoOverridesReturnsStepsVerbatim(t *testing.T) {
	steps := []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentActive, Target: &wt.Target{Kind: wt.TargetHRZone, Low: intp(2), High: intp(2)}}}
	out := applyOverrides(steps, nil, nil)
	require.Len(t, out, 1)
	assert.Equal(t, wt.TargetHRZone, out[0].Target.Kind)
}

func timeDur(sec int) wt.Duration { return wt.Duration{Kind: wt.DurationTime, Seconds: intp(sec)} }

func timeDurP(sec int) *wt.Duration { d := timeDur(sec); return &d }

func TestValidateDurationOverrides(t *testing.T) {
	t.Run("valid time override accepted", func(t *testing.T) {
		require.NoError(t, validateDurationOverrides([]SlotDurationOverride{{Intent: wt.IntentActive, Duration: timeDur(3600)}}))
	})
	t.Run("valid distance override accepted", func(t *testing.T) {
		d := wt.Duration{Kind: wt.DurationDistance, Meters: intp(10000)}
		require.NoError(t, validateDurationOverrides([]SlotDurationOverride{{Intent: wt.IntentActive, Duration: d}}))
	})
	t.Run("unknown intent rejected", func(t *testing.T) {
		assert.ErrorIs(t, validateDurationOverrides([]SlotDurationOverride{{Intent: "sprint", Duration: timeDur(600)}}), ErrOverrideIntentInvalid)
	})
	t.Run("duplicate intent rejected", func(t *testing.T) {
		err := validateDurationOverrides([]SlotDurationOverride{
			{Intent: wt.IntentInterval, Duration: timeDur(180)},
			{Intent: wt.IntentInterval, Duration: timeDur(240)},
		})
		assert.ErrorIs(t, err, ErrOverrideDuplicate)
	})
	t.Run("open kind rejected", func(t *testing.T) {
		assert.ErrorIs(t, validateDurationOverrides([]SlotDurationOverride{{Intent: wt.IntentActive, Duration: wt.Duration{Kind: wt.DurationOpen}}}), ErrOverrideDurationInvalid)
	})
	t.Run("lap_button kind rejected", func(t *testing.T) {
		assert.ErrorIs(t, validateDurationOverrides([]SlotDurationOverride{{Intent: wt.IntentActive, Duration: wt.Duration{Kind: wt.DurationLapButton}}}), ErrOverrideDurationInvalid)
	})
	t.Run("non-positive seconds rejected", func(t *testing.T) {
		assert.ErrorIs(t, validateDurationOverrides([]SlotDurationOverride{{Intent: wt.IntentActive, Duration: timeDur(0)}}), ErrOverrideDurationInvalid)
	})
	t.Run("non-positive meters rejected", func(t *testing.T) {
		d := wt.Duration{Kind: wt.DurationDistance, Meters: intp(0)}
		assert.ErrorIs(t, validateDurationOverrides([]SlotDurationOverride{{Intent: wt.IntentActive, Duration: d}}), ErrOverrideDurationInvalid)
	})
	t.Run("unknown kind rejected", func(t *testing.T) {
		assert.ErrorIs(t, validateDurationOverrides([]SlotDurationOverride{{Intent: wt.IntentActive, Duration: wt.Duration{Kind: "forever"}}}), ErrOverrideDurationInvalid)
	})
	t.Run("empty list ok", func(t *testing.T) {
		require.NoError(t, validateDurationOverrides(nil))
	})
}

func TestApplyOverrides_DurationReplacesMatchingIntentAndComposes(t *testing.T) {
	steps := []wt.Step{
		{Type: wt.NodeStep, Intent: wt.IntentWarmup, Duration: timeDurP(600), Target: &wt.Target{Kind: wt.TargetHRZone, Low: intp(1), High: intp(1)}},
		{Type: wt.NodeStep, Intent: wt.IntentActive, Duration: timeDurP(3300), Target: &wt.Target{Kind: wt.TargetHRZone, Low: intp(3), High: intp(3)}},
		{Type: wt.NodeStep, Intent: wt.IntentCooldown, Duration: timeDurP(600), Target: &wt.Target{Kind: wt.TargetHRZone, Low: intp(1), High: intp(1)}},
	}
	durations := []SlotDurationOverride{{Intent: wt.IntentActive, Duration: timeDur(3600)}}
	targets := []SlotTargetOverride{{Intent: wt.IntentActive, Target: paceTarget(300, 300)}}
	out := applyOverrides(steps, targets, durations)

	// active step: both duration and target overridden (compose).
	assert.Equal(t, 3600, *out[1].Duration.Seconds)
	assert.Equal(t, wt.TargetPace, out[1].Target.Kind)
	// warmup/cooldown durations untouched.
	assert.Equal(t, 600, *out[0].Duration.Seconds)
	assert.Equal(t, 600, *out[2].Duration.Seconds)
	// original template slice not mutated.
	assert.Equal(t, 3300, *steps[1].Duration.Seconds)
}

func TestEffectiveSessionDurationSec(t *testing.T) {
	t.Run("sums time-bounded leaves and multiplies repeats", func(t *testing.T) {
		steps := []wt.Step{
			{Type: wt.NodeStep, Intent: wt.IntentWarmup, Duration: timeDurP(600)},
			{Type: wt.NodeRepeat, Count: 5, Steps: []wt.Step{
				{Type: wt.NodeStep, Intent: wt.IntentInterval, Duration: timeDurP(180)},
				{Type: wt.NodeStep, Intent: wt.IntentRecovery, Duration: timeDurP(120)},
			}},
			{Type: wt.NodeStep, Intent: wt.IntentCooldown, Duration: timeDurP(600)},
		}
		got, ok := effectiveSessionDurationSec(steps)
		require.True(t, ok)
		assert.Equal(t, 600+5*(180+120)+600, got)
	})
	t.Run("not derivable when a leaf is distance-bounded", func(t *testing.T) {
		steps := []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentActive, Duration: &wt.Duration{Kind: wt.DurationDistance, Meters: intp(10000)}}}
		_, ok := effectiveSessionDurationSec(steps)
		assert.False(t, ok)
	})
	t.Run("not derivable when a leaf is open", func(t *testing.T) {
		steps := []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentActive, Duration: &wt.Duration{Kind: wt.DurationOpen}}}
		_, ok := effectiveSessionDurationSec(steps)
		assert.False(t, ok)
	})
	t.Run("empty program is not derivable", func(t *testing.T) {
		_, ok := effectiveSessionDurationSec(nil)
		assert.False(t, ok)
	})
}
