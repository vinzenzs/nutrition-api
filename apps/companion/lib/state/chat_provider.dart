import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/net/idempotency.dart';
import '../domain/chat.dart';
import 'app_providers.dart';

/// One active conversation. The streaming bubble's text lives in
/// [streamingText] while a turn is in flight; tool chips for the current turn
/// live in [tools]. [error] holds the last turn's failure code (retryable).
class ChatState {
  final List<ChatMessage> messages;
  final String? streamingText;
  final List<ChatToolEvent> tools;
  final String? error;

  const ChatState({
    this.messages = const [],
    this.streamingText,
    this.tools = const [],
    this.error,
  });

  bool get streaming => streamingText != null;

  ChatState copyWith({
    List<ChatMessage>? messages,
    String? streamingText,
    bool clearStreaming = false,
    List<ChatToolEvent>? tools,
    String? error,
    bool clearError = false,
  }) {
    return ChatState(
      messages: messages ?? this.messages,
      streamingText: clearStreaming ? null : (streamingText ?? this.streamingText),
      tools: tools ?? this.tools,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

/// History sent to the backend is capped client-side; the backend truncates
/// further. System messages are never sent (we only hold user/assistant).
const _maxHistory = 40;

class ChatNotifier extends Notifier<ChatState> {
  late String _conversationId;
  String? _lastUserText;

  @override
  ChatState build() {
    _conversationId = newIdempotencyKey();
    return const ChatState();
  }

  /// Starts a fresh conversation. Old messages stay in Drift for scrollback.
  void newChat() {
    _conversationId = newIdempotencyKey();
    _lastUserText = null;
    state = const ChatState();
  }

  /// Sends [text] as a user turn and streams the assistant reply.
  Future<void> send(String text) async {
    final trimmed = text.trim();
    if (trimmed.isEmpty || state.streaming) return;
    final user = ChatMessage(
      id: newIdempotencyKey(),
      role: ChatRole.user,
      content: trimmed,
      createdAt: DateTime.now(),
    );
    _lastUserText = trimmed;
    state = state.copyWith(
      messages: [...state.messages, user],
      streamingText: '',
      tools: const [],
      clearError: true,
    );
    await _persist(user);
    await _runStream();
  }

  /// Re-runs the last user turn after a failure (idempotent on the backend).
  Future<void> retry() async {
    if (_lastUserText == null || state.streaming) return;
    state = state.copyWith(streamingText: '', tools: const [], clearError: true);
    await _runStream();
  }

  Future<void> _runStream() async {
    final history = state.messages.length > _maxHistory
        ? state.messages.sublist(state.messages.length - _maxHistory)
        : state.messages;
    final buffer = StringBuffer();
    final tools = <ChatToolEvent>[];
    try {
      await for (final ev in ref.read(chatClientProvider).stream(history)) {
        switch (ev) {
          case ChatTextEvent(:final text):
            buffer.write(text);
            state = state.copyWith(streamingText: buffer.toString());
          case ChatToolEvent():
            tools.add(ev);
            state = state.copyWith(tools: List.of(tools));
          case ChatDoneEvent(:final message):
            await _finalize(message.isNotEmpty ? message : buffer.toString());
            return;
          case ChatErrorEvent(:final code):
            state = state.copyWith(clearStreaming: true, error: code);
            return;
        }
      }
      // Stream ended without a done/error event — treat as a dropped turn.
      if (state.streaming) {
        state = state.copyWith(clearStreaming: true, error: 'stream_dropped');
      }
    } catch (e) {
      state = state.copyWith(clearStreaming: true, error: 'stream_dropped');
    }
  }

  Future<void> _finalize(String content) async {
    final assistant = ChatMessage(
      id: newIdempotencyKey(),
      role: ChatRole.assistant,
      content: content,
      createdAt: DateTime.now(),
    );
    state = state.copyWith(
      messages: [...state.messages, assistant],
      clearStreaming: true,
      tools: const [],
    );
    await _persist(assistant);
    await ref.read(appDatabaseProvider).chatMessagesDao.pruneConversations();
  }

  Future<void> _persist(ChatMessage m) {
    return ref.read(appDatabaseProvider).chatMessagesDao.insertMessage(
          id: m.id,
          conversationId: _conversationId,
          role: chatRoleName(m.role),
          content: m.content,
          createdAt: m.createdAt,
        );
  }
}

final chatProvider =
    NotifierProvider<ChatNotifier, ChatState>(ChatNotifier.new);
