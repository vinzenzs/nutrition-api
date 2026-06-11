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
  final String name;
  final String status;
  final String summary;
  ChatToolEvent({required this.name, required this.status, required this.summary});
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
