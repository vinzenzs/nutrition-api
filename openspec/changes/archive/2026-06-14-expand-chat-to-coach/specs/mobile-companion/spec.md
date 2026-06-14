## MODIFIED Requirements

### Requirement: The mobile companion is a focused supplement to the agent, not a replacement

The mobile companion app SHALL implement exactly four screens (Today, Camera, Recent, Chat) plus a Settings sheet and a shopping list screen reachable from the Today header. The Chat screen is the in-app coach backed by the server's `nutrition-chat` capability — it spans nutrition planning and endurance-training coaching, including the full write surface, with consequential (training/goal/destructive) writes gated behind an in-app confirmation. The app SHALL NOT include an in-app recipe builder or a generalized product search experience.

#### Scenario: Four screens after chat activation

- **WHEN** the user opens the app
- **THEN** the bottom navigation surfaces Today, Camera, Recent, and Chat as the four primary destinations
- **AND** Settings and the shopping list are reachable from the Today screen's top-right
- **AND** no recipe builder or all-products search screen exists

#### Scenario: Chat is the coach

- **WHEN** the user asks the Chat screen a training, recovery, or fueling question
- **THEN** the app renders the assistant's grounded coaching reply as ordinary chat content
- **AND** the assistant is not limited to nutrition planning (no redirect-to-desktop-coach behavior is expected)

## ADDED Requirements

### Requirement: The Chat screen confirms consequential writes before they fire

The Chat screen SHALL render a pending write confirmation as a card listing each pending write's human `preview` with a **per-item** toggle, an "Apply selected" action, and a reject affordance, and SHALL NOT let any `write-confirm` action take effect until the user applies it. The card SHALL be rendered identically whether it arrives as a live `proposal` SSE event or is reconstructed from a session's `pending_confirmation` on (re)open, so it survives the app being backgrounded or killed. On apply, the app SHALL call `POST /chat/sessions/{id}/confirm` with the per-call decisions and resume rendering the streamed continuation on the same screen. The composer SHALL remain usable while a card is pending: sending a new message implicitly rejects the pending writes (the server resolves them) and proceeds.

#### Scenario: A proposed training write shows a per-item confirm card

- **WHEN** the coach proposes one or more `write-confirm` actions (e.g. scheduling workouts or changing a goal)
- **THEN** the Chat screen shows a card with one toggle per pending write and its preview
- **AND** nothing is written until the user taps "Apply selected"

#### Scenario: Applying a subset writes only the selected actions

- **WHEN** the user deselects one item and taps "Apply selected"
- **THEN** the app POSTs per-call decisions approving only the selected items and rejecting the rest
- **AND** renders the resumed stream (tool-status chips, then text) inline

#### Scenario: Typing instead of confirming implicitly rejects

- **WHEN** the user ignores the card and sends a new message
- **THEN** the pending writes are implicitly rejected server-side and the new turn proceeds, with nothing written

#### Scenario: Killing the app mid-confirmation loses nothing

- **WHEN** the app is closed while a card is pending and later reopened to that session
- **THEN** the card is reconstructed from the session's `pending_confirmation` and the user can still apply or reject
