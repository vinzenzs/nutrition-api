package mcpserver

// GoalRange is a min/max nutrient target band. It was defined in tools_goals.go
// until the goals + training-phases domains were ported onto the shared
// agenttools registry (unify-mcp-tool-registry); the still-bespoke
// goal-overrides domain (tools_goal_overrides.go) still references it. Remove
// this once goal-overrides is ported (it carries its own agenttools.GoalRange).
type GoalRange struct {
	Min *float64 `json:"min,omitempty" jsonschema:"minimum target value"`
	Max *float64 `json:"max,omitempty" jsonschema:"maximum target value"`
}
