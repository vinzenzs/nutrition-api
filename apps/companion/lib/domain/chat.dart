// Chat domain types: transcript messages and the four streamed event types the
// backend `POST /chat` SSE emits (text | tool | done | error).

enum ChatRole { user, assistant }

String chatRoleName(ChatRole r) => r == ChatRole.user ? 'user' : 'assistant';

ChatRole chatRoleFrom(String s) =>
    s == 'user' ? ChatRole.user : ChatRole.assistant;

class ChatMessage {
  final String id;
  final ChatRole role;
  final String content;
  final DateTime createdAt;

  ChatMessage({
    required this.id,
    required this.role,
    required this.content,
    required this.createdAt,
  });
}

/// A streamed event from the chat SSE response.
sealed class ChatEvent {}

/// An assistant text delta.
class ChatTextEvent extends ChatEvent {
  final String text;
  ChatTextEvent(this.text);
}

/// A tool's lifecycle: status is started | ok | error; summary is a short
/// human string (never raw bodies).
class ChatToolEvent extends ChatEvent {
  /// The upstream tool_use id; a call's started and ok/error events share it,
  /// so the UI coalesces them into one chip.
  final String id;
  final String name;
  final String status;
  final String summary;
  ChatToolEvent({required this.id, required this.name, required this.status, required this.summary});
  bool get isError => status == 'error';
}

/// Terminates a successful stream with the full final message.
class ChatDoneEvent extends ChatEvent {
  final String message;
  final String stopReason;
  ChatDoneEvent({required this.message, required this.stopReason});
}

/// Terminates the stream with a typed code (e.g. chat_unavailable).
class ChatErrorEvent extends ChatEvent {
  final String code;
  final String message;
  ChatErrorEvent({required this.code, required this.message});
}

/// One row in the session-history list — the `GET /chat/sessions` header
/// (no transcript). Title is null when the backend hasn't named it yet.
class ChatSessionSummary {
  final String id;
  final String? title;
  final DateTime lastMessageAt;
  final DateTime createdAt;

  ChatSessionSummary({
    required this.id,
    this.title,
    required this.lastMessageAt,
    required this.createdAt,
  });

  factory ChatSessionSummary.fromJson(Map<String, dynamic> j) =>
      ChatSessionSummary(
        id: j['id'] as String,
        title: j['title'] as String?,
        lastMessageAt:
            DateTime.parse(j['last_message_at'] as String).toLocal(),
        createdAt: DateTime.parse(j['created_at'] as String).toLocal(),
      );
}

/// A reopened session: its header plus the reconstructed visible transcript.
class ChatSessionDetail {
  final ChatSessionSummary summary;
  final List<ChatMessage> messages;
  ChatSessionDetail({required this.summary, required this.messages});
}

/// A typed failure from the `/chat/sessions` read/manage calls.
class ChatSessionException implements Exception {
  final String code;
  const ChatSessionException(this.code);
  @override
  String toString() => 'ChatSessionException($code)';
}

/// Reconstructs visible chat bubbles from a session's stored turns. Each turn is
/// `{role, content}` where `content` is the verbatim Anthropic value: a JSON
/// string (plain user text) or a content-block array. User string turns become
/// user bubbles; assistant turns become a bubble from their `text` blocks;
/// `tool_use`-only assistant turns and `tool_result` user turns are dropped.
/// Pure and deterministic — timestamps are synthetic (unused by the transcript).
List<ChatMessage> reconstructTranscript(List<dynamic> turns) {
  final out = <ChatMessage>[];
  final epoch = DateTime.fromMillisecondsSinceEpoch(0);
  var i = 0;
  for (final raw in turns) {
    if (raw is Map) {
      final role = raw['role'];
      final content = raw['content'];
      if (role == 'user' && content is String) {
        out.add(ChatMessage(
            id: 'hist-$i', role: ChatRole.user, content: content, createdAt: epoch));
      } else if (role == 'assistant') {
        final text = _assistantText(content);
        if (text.isNotEmpty) {
          out.add(ChatMessage(
              id: 'hist-$i',
              role: ChatRole.assistant,
              content: text,
              createdAt: epoch));
        }
      }
      // user array content (tool_result) and assistant tool_use-only turns: skip.
    }
    i++;
  }
  return out;
}

/// Concatenates the `text` blocks of an assistant content value.
String _assistantText(dynamic content) {
  if (content is String) return content; // defensive — shouldn't happen
  if (content is! List) return '';
  final b = StringBuffer();
  for (final block in content) {
    if (block is Map && block['type'] == 'text' && block['text'] is String) {
      b.write(block['text'] as String);
    }
  }
  return b.toString();
}
