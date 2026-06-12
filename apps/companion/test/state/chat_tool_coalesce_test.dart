import 'package:flutter_test/flutter_test.dart';
import 'package:nutrition_companion/domain/chat.dart';
import 'package:nutrition_companion/state/chat_provider.dart';

ChatToolEvent _tool(String id, String name, String status, {String summary = ''}) =>
    ChatToolEvent(id: id, name: name, status: status, summary: summary);

void main() {
  test('a call started then ok yields one chip ending in done', () {
    final tools = <ChatToolEvent>[];
    upsertToolEvent(tools, _tool('t1', 'daily_context', 'started'));
    expect(tools, hasLength(1));
    expect(tools.single.status, 'started');

    upsertToolEvent(tools, _tool('t1', 'daily_context', 'ok'));
    expect(tools, hasLength(1), reason: 'started + ok coalesce into one chip');
    expect(tools.single.status, 'ok');
    expect(tools.single.name, 'daily_context');
  });

  test('an error terminal updates the same chip and flags it', () {
    final tools = <ChatToolEvent>[];
    upsertToolEvent(tools, _tool('t1', 'plan_carb_load', 'started'));
    upsertToolEvent(tools, _tool('t1', 'plan_carb_load', 'error', summary: 'bad date'));
    expect(tools, hasLength(1));
    expect(tools.single.isError, isTrue);
    expect(tools.single.summary, 'bad date');
  });

  test('distinct ids render distinct chips even for the same tool name', () {
    final tools = <ChatToolEvent>[];
    upsertToolEvent(tools, _tool('t1', 'list_workouts', 'started'));
    upsertToolEvent(tools, _tool('t2', 'list_workouts', 'started'));
    upsertToolEvent(tools, _tool('t1', 'list_workouts', 'ok'));
    upsertToolEvent(tools, _tool('t2', 'list_workouts', 'ok'));
    expect(tools, hasLength(2), reason: 'two calls of the same tool stay two chips');
    expect(tools.map((t) => t.id).toList(), ['t1', 't2'], reason: 'first-seen order preserved');
    expect(tools.every((t) => t.status == 'ok'), isTrue);
  });

  test('order is the order calls first appear', () {
    final tools = <ChatToolEvent>[];
    upsertToolEvent(tools, _tool('a', 'first', 'started'));
    upsertToolEvent(tools, _tool('b', 'second', 'started'));
    upsertToolEvent(tools, _tool('a', 'first', 'ok')); // updates in place, no reorder
    expect(tools.map((t) => t.name).toList(), ['first', 'second']);
  });

  test('an empty id falls back to append (no accidental coalescing)', () {
    final tools = <ChatToolEvent>[];
    upsertToolEvent(tools, _tool('', 'x', 'started'));
    upsertToolEvent(tools, _tool('', 'y', 'started'));
    expect(tools, hasLength(2));
  });
}
