import 'package:flutter_test/flutter_test.dart';
import 'package:kazper/domain/chat.dart';

void main() {
  test('ChatPending.fromJson parses turn id and calls', () {
    final p = ChatPending.fromJson({
      'turn_id': 'turn_xyz',
      'calls': [
        {
          'tool_id': 'c1',
          'name': 'set_daily_goal_override',
          'tier': 'write-confirm',
          'preview': 'Set goal override for 2026-06-20',
        },
      ],
    });
    expect(p.turnId, 'turn_xyz');
    expect(p.calls.single.name, 'set_daily_goal_override');
    expect(p.calls.single.preview, 'Set goal override for 2026-06-20');
  });

  test('session summary carries the awaiting_confirmation flag', () {
    final flagged = ChatSessionSummary.fromJson({
      'id': 's1',
      'last_message_at': '2026-06-14T10:00:00Z',
      'created_at': '2026-06-14T09:00:00Z',
      'awaiting_confirmation': true,
    });
    expect(flagged.awaitingConfirmation, isTrue);

    // Absent flag defaults to false (the field is omitted when not paused).
    final normal = ChatSessionSummary.fromJson({
      'id': 's2',
      'last_message_at': '2026-06-14T10:00:00Z',
      'created_at': '2026-06-14T09:00:00Z',
    });
    expect(normal.awaitingConfirmation, isFalse);
  });
}
