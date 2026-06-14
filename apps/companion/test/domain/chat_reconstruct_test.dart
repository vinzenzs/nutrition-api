import 'package:flutter_test/flutter_test.dart';
import 'package:kazper/domain/chat.dart';

void main() {
  test('reconstructs text bubbles, dropping tool turns', () {
    // Mirrors a stored session: user text, assistant tool_use, tool_result,
    // final assistant text.
    final turns = <dynamic>[
      {'role': 'user', 'content': 'what should I eat today?'},
      {
        'role': 'assistant',
        'content': [
          {'type': 'text', 'text': 'Let me check.'},
          {
            'type': 'tool_use',
            'id': 't1',
            'name': 'get_daily_context',
            'input': {'date': '2026-06-12'}
          },
        ],
      },
      {
        'role': 'user',
        'content': [
          {'type': 'tool_result', 'tool_use_id': 't1', 'content': '{...}'}
        ],
      },
      {
        'role': 'assistant',
        'content': [
          {'type': 'text', 'text': 'Three options: A, B, C.'}
        ],
      },
    ];

    final msgs = reconstructTranscript(turns);

    expect(msgs, hasLength(3));
    expect(msgs[0].role, ChatRole.user);
    expect(msgs[0].content, 'what should I eat today?');
    expect(msgs[1].role, ChatRole.assistant);
    expect(msgs[1].content, 'Let me check.'); // text kept, tool_use dropped
    expect(msgs[2].role, ChatRole.assistant);
    expect(msgs[2].content, 'Three options: A, B, C.');
  });

  test('drops an assistant turn with only a tool_use block', () {
    final turns = <dynamic>[
      {
        'role': 'assistant',
        'content': [
          {'type': 'tool_use', 'id': 't1', 'name': 'x', 'input': {}}
        ],
      },
    ];
    expect(reconstructTranscript(turns), isEmpty);
  });

  test('empty transcript yields no bubbles', () {
    expect(reconstructTranscript(const []), isEmpty);
  });
}
