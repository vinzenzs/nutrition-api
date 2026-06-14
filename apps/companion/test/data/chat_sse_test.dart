import 'package:flutter_test/flutter_test.dart';
import 'package:kazper/data/net/chat_client.dart';
import 'package:kazper/domain/chat.dart';

/// Feeds raw SSE lines through the pure parser.
Stream<String> _lines(List<String> raw) => Stream.fromIterable(raw);

void main() {
  test('parses all four event types', () async {
    final events = await parseSseLines(_lines([
      'event: text',
      'data: {"text":"Hi "}',
      '',
      'event: tool',
      'data: {"name":"add_shopping_items","status":"ok","summary":"Added 14 items"}',
      '',
      'event: text',
      'data: {"text":"there"}',
      '',
      'event: done',
      'data: {"message":"Hi there","stop_reason":"end_turn"}',
      '',
    ])).toList();

    expect(events, hasLength(4));
    expect((events[0] as ChatTextEvent).text, 'Hi ');
    final tool = events[1] as ChatToolEvent;
    expect(tool.name, 'add_shopping_items');
    expect(tool.status, 'ok');
    expect(tool.summary, 'Added 14 items');
    expect((events[2] as ChatTextEvent).text, 'there');
    final done = events[3] as ChatDoneEvent;
    expect(done.message, 'Hi there');
    expect(done.stopReason, 'end_turn');
  });

  test('error event is decoded', () async {
    final events = await parseSseLines(_lines([
      'event: error',
      'data: {"code":"upstream_unavailable","message":"down"}',
      '',
    ])).toList();
    final err = events.single as ChatErrorEvent;
    expect(err.code, 'upstream_unavailable');
  });

  test('tool error chip flagged', () async {
    final events = await parseSseLines(_lines([
      'event: tool',
      'data: {"name":"plan_carb_load","status":"error","summary":"bad date"}',
      '',
    ])).toList();
    expect((events.single as ChatToolEvent).isError, isTrue);
  });

  test('mid-stream drop: a trailing event with no blank line still flushes', () async {
    // Simulates the connection dropping right after a complete data line but
    // before the terminating blank line.
    final events = await parseSseLines(_lines([
      'event: text',
      'data: {"text":"partial"}',
    ])).toList();
    expect((events.single as ChatTextEvent).text, 'partial');
  });

  test('unknown event types are skipped', () async {
    final events = await parseSseLines(_lines([
      'event: ping',
      'data: {}',
      '',
      'event: text',
      'data: {"text":"ok"}',
      '',
    ])).toList();
    expect(events, hasLength(1));
    expect((events.single as ChatTextEvent).text, 'ok');
  });

  test('proposal event then awaiting_confirmation done', () async {
    final events = await parseSseLines(_lines([
      'event: text',
      'data: {"text":"I will schedule your ride."}',
      '',
      'event: proposal',
      'data: {"turn_id":"turn_abc","calls":[{"tool_id":"c1","name":"schedule_workout","tier":"write-confirm","preview":"Schedule a ride on 2026-06-20"}]}',
      '',
      'event: done',
      'data: {"message":"I will schedule your ride.","stop_reason":"awaiting_confirmation"}',
      '',
    ])).toList();

    expect(events, hasLength(3));
    final proposal = events[1] as ChatProposalEvent;
    expect(proposal.pending.turnId, 'turn_abc');
    expect(proposal.pending.calls, hasLength(1));
    final call = proposal.pending.calls.single;
    expect(call.toolId, 'c1');
    expect(call.name, 'schedule_workout');
    expect(call.tier, 'write-confirm');
    expect(call.preview, 'Schedule a ride on 2026-06-20');

    final done = events[2] as ChatDoneEvent;
    expect(done.awaitingConfirmation, isTrue);
  });
}
