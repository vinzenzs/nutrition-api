import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kazper/data/auth/token_store.dart';
import 'package:kazper/data/net/chat_client.dart';
import 'package:kazper/domain/chat.dart';
import 'package:kazper/state/app_providers.dart';
import 'package:kazper/state/chat_provider.dart';
import 'package:kazper/state/sessions_provider.dart';

class _StubTokenStore implements TokenStore {
  @override
  Future<String?> getToken() async => 't';
  @override
  Future<String?> getBaseUrl() async => 'http://x';
  @override
  Future<void> pair({required String baseUrl, required String token}) async {}
  @override
  Future<void> clear() async {}
}

/// A ChatClient whose session methods are in-memory; networking is bypassed.
class FakeChatClient extends ChatClient {
  FakeChatClient() : super(tokenStore: _StubTokenStore());

  List<ChatSessionSummary> sessions = [];
  final List<String> deleted = [];
  final Map<String, String> renamed = {};
  ChatSessionDetail? detail;

  @override
  Future<List<ChatSessionSummary>> listSessions() async => sessions;

  @override
  Future<ChatSessionDetail> getSession(String id) async =>
      detail ??
      ChatSessionDetail(summary: _summary(id), messages: const []);

  @override
  Future<void> deleteSession(String id) async {
    deleted.add(id);
    sessions = sessions.where((s) => s.id != id).toList();
  }

  @override
  Future<void> renameSession(String id, String title) async {
    renamed[id] = title;
    sessions = [
      for (final s in sessions)
        if (s.id == id)
          ChatSessionSummary(
              id: s.id,
              title: title.isEmpty ? null : title,
              lastMessageAt: s.lastMessageAt,
              createdAt: s.createdAt)
        else
          s,
    ];
  }
}

ChatSessionSummary _summary(String id, {String? title, int minsAgo = 0}) =>
    ChatSessionSummary(
      id: id,
      title: title,
      lastMessageAt: DateTime.now().subtract(Duration(minutes: minsAgo)),
      createdAt: DateTime.now().subtract(Duration(minutes: minsAgo)),
    );

ProviderContainer _container(FakeChatClient client) {
  final c = ProviderContainer(
    overrides: [chatClientProvider.overrideWithValue(client)],
  );
  addTearDown(c.dispose);
  return c;
}

Future<SessionsState> _settle(ProviderContainer c) async {
  for (var i = 0; i < 50; i++) {
    await Future<void>.delayed(const Duration(milliseconds: 10));
    final s = c.read(sessionsProvider);
    if (!s.loading) return s;
  }
  throw StateError('sessions never settled');
}

void main() {
  test('loads sessions on build', () async {
    final client = FakeChatClient()
      ..sessions = [_summary('a', title: 'A'), _summary('b')];
    final c = _container(client);
    c.read(sessionsProvider);

    final s = await _settle(c);
    expect(s.sessions.map((e) => e.id), ['a', 'b']);
    expect(s.error, isNull);
  });

  test('load failure surfaces a retryable error', () async {
    final client = _ThrowingClient();
    final c = _container(client);
    c.read(sessionsProvider);

    final s = await _settle(c);
    expect(s.error, 'load_failed');
    expect(s.sessions, isEmpty);
  });

  test('delete removes optimistically and calls the client', () async {
    final client = FakeChatClient()
      ..sessions = [_summary('a'), _summary('b')];
    final c = _container(client);
    c.read(sessionsProvider);
    await _settle(c);

    await c.read(sessionsProvider.notifier).delete('a');
    expect(client.deleted, ['a']);
    expect(c.read(sessionsProvider).sessions.map((e) => e.id), ['b']);
  });

  test('rename calls the client and refreshes', () async {
    final client = FakeChatClient()..sessions = [_summary('a', title: 'old')];
    final c = _container(client);
    c.read(sessionsProvider);
    await _settle(c);

    await c.read(sessionsProvider.notifier).rename('a', 'new');
    expect(client.renamed['a'], 'new');
    expect(c.read(sessionsProvider).sessions.single.title, 'new');
  });

  test('openSession adopts the session id', () async {
    final client = FakeChatClient()
      ..detail = ChatSessionDetail(
        summary: _summary('a', title: 'A'),
        messages: [
          ChatMessage(
              id: 'm1',
              role: ChatRole.user,
              content: 'hi',
              createdAt: DateTime.now())
        ],
      );
    final c = _container(client);

    final ok = await c.read(chatProvider.notifier).openSession(_summary('a'));
    expect(ok, isTrue);
    expect(c.read(chatProvider.notifier).activeSessionId, 'a');
    expect(c.read(chatProvider).messages.single.content, 'hi');
  });

  test('openSession rebuilds a pending proposal card on cold-open', () async {
    final client = FakeChatClient()
      ..detail = ChatSessionDetail(
        summary: _summary('a', title: 'Paused'),
        messages: const [],
        pending: ChatPending(turnId: 'turn_1', calls: [
          ChatPendingCall(
              toolId: 'c1',
              name: 'schedule_workout',
              tier: 'write-confirm',
              preview: 'Schedule a ride on 2026-06-20'),
        ]),
      );
    final c = _container(client);

    await c.read(chatProvider.notifier).openSession(_summary('a'));
    final pending = c.read(chatProvider).pending;
    expect(pending, isNotNull);
    expect(pending!.calls.single.preview, 'Schedule a ride on 2026-06-20');
  });

  test('deleting the active session resets the chat', () async {
    final client = FakeChatClient()..sessions = [_summary('a')];
    final c = _container(client);
    c.read(sessionsProvider);
    await _settle(c);
    await c.read(chatProvider.notifier).openSession(_summary('a'));
    expect(c.read(chatProvider.notifier).activeSessionId, 'a');

    await c.read(sessionsProvider.notifier).delete('a');
    expect(c.read(chatProvider.notifier).activeSessionId, isNull);
  });
}

class _ThrowingClient extends FakeChatClient {
  @override
  Future<List<ChatSessionSummary>> listSessions() async =>
      throw const ChatSessionException('http_500');
}
