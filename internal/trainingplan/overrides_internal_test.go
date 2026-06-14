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
	out := applyOverrides(steps, []SlotTargetOverride{{Intent: wt.IntentInterval, Target: pace}})

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
	out := applyOverrides(steps, nil)
	require.Len(t, out, 1)
	assert.Equal(t, wt.TargetHRZone, out[0].Target.Kind)
}
