import 'dart:convert';

import 'package:http/http.dart' as http;

import '../../domain/chat.dart';
import '../auth/token_store.dart';

/// Streams a chat turn over the backend SSE endpoint. The backend is
/// session-backed: each call POSTs `{session_id, message}` (the server holds
/// the transcript) and the response is a `text/event-stream` of four event
/// types. Hand-rolled (no SSE package) per design D2 — the parser is pulled out
/// as a pure function for testing.
class ChatClient {
  final TokenStore tokenStore;
  final http.Client _http;

  ChatClient({required this.tokenStore, http.Client? client})
      : _http = client ?? http.Client();

  /// Opens a new server-side conversation, returning its session id (or null on
  /// a network/not-paired failure — the caller surfaces a retryable error).
  Future<String?> createSession() async {
    final baseUrl = await tokenStore.getBaseUrl();
    final token = await tokenStore.getToken();
    if (baseUrl == null || token == null) return null;
    try {
      final resp = await _http.post(
        Uri.parse('$baseUrl/chat/sessions'),
        headers: {
          'Authorization': 'Bearer $token',
          'Content-Type': 'application/json',
        },
        body: jsonEncode(const <String, dynamic>{}),
      );
      if (resp.statusCode != 201) return null;
      final j = jsonDecode(resp.body);
      if (j is Map && j['id'] is String) return j['id'] as String;
      return null;
    } catch (_) {
      return null;
    }
  }

  /// Lists past sessions newest-first (`GET /chat/sessions`). Online-only —
  /// throws [ChatSessionException] on a network/not-paired/HTTP failure.
  Future<List<ChatSessionSummary>> listSessions() async {
    final (baseUrl, token) = await _creds();
    final resp = await _get('$baseUrl/chat/sessions', token);
    if (resp.statusCode != 200) {
      throw ChatSessionException('http_${resp.statusCode}');
    }
    final j = jsonDecode(resp.body);
    final list = j is Map ? j['sessions'] : null;
    if (list is! List) return const [];
    return [
      for (final s in list)
        if (s is Map<String, dynamic>) ChatSessionSummary.fromJson(s),
    ];
  }

  /// Fetches one session with its transcript, reconstructed to visible bubbles
  /// (`GET /chat/sessions/{id}`).
  Future<ChatSessionDetail> getSession(String id) async {
    final (baseUrl, token) = await _creds();
    final resp = await _get('$baseUrl/chat/sessions/$id', token);
    if (resp.statusCode != 200) {
      throw ChatSessionException('http_${resp.statusCode}');
    }
    final j = jsonDecode(resp.body) as Map<String, dynamic>;
    return ChatSessionDetail(
      summary: ChatSessionSummary.fromJson(j),
      messages: reconstructTranscript((j['messages'] as List?) ?? const []),
    );
  }

  /// Renames a session (`PATCH /chat/sessions/{id}`); an empty title clears it.
  Future<void> renameSession(String id, String title) async {
    final (baseUrl, token) = await _creds();
    final resp = await _http.patch(
      Uri.parse('$baseUrl/chat/sessions/$id'),
      headers: {
        'Authorization': 'Bearer $token',
        'Content-Type': 'application/json',
      },
      body: jsonEncode({'title': title}),
    );
    if (resp.statusCode != 200) {
      throw ChatSessionException('http_${resp.statusCode}');
    }
  }

  /// Deletes a session (`DELETE /chat/sessions/{id}`).
  Future<void> deleteSession(String id) async {
    final (baseUrl, token) = await _creds();
    final resp = await _http.delete(
      Uri.parse('$baseUrl/chat/sessions/$id'),
      headers: {'Authorization': 'Bearer $token'},
    );
    if (resp.statusCode != 204 && resp.statusCode != 200) {
      throw ChatSessionException('http_${resp.statusCode}');
    }
  }

  /// Resolves baseUrl + token or throws `not_paired`.
  Future<(String, String)> _creds() async {
    final baseUrl = await tokenStore.getBaseUrl();
    final token = await tokenStore.getToken();
    if (baseUrl == null || token == null) {
      throw const ChatSessionException('not_paired');
    }
    return (baseUrl, token);
  }

  Future<http.Response> _get(String url, String token) {
    return _http.get(Uri.parse(url), headers: {'Authorization': 'Bearer $token'});
  }

  /// Streams one turn: posts [message] against the existing [sessionId]; the
  /// server loads prior turns and persists the new ones.
  Stream<ChatEvent> stream({
    required String sessionId,
    required String message,
  }) async* {
    final baseUrl = await tokenStore.getBaseUrl();
    final token = await tokenStore.getToken();
    if (baseUrl == null || token == null) {
      yield ChatErrorEvent(code: 'not_paired', message: 'Not paired');
      return;
    }
    final req = http.Request('POST', Uri.parse('$baseUrl/chat'))
      ..headers['Authorization'] = 'Bearer $token'
      ..headers['Content-Type'] = 'application/json'
      ..headers['Accept'] = 'text/event-stream'
      ..body = jsonEncode({'session_id': sessionId, 'message': message});

    http.StreamedResponse resp;
    try {
      resp = await _http.send(req);
    } catch (e) {
      yield ChatErrorEvent(code: 'network', message: e.toString());
      return;
    }

    if (resp.statusCode != 200) {
      // Non-stream errors come back as JSON {"error": "<code>"}.
      final body = await resp.stream.bytesToString();
      var code = 'http_${resp.statusCode}';
      try {
        final j = jsonDecode(body);
        if (j is Map && j['error'] is String) code = j['error'] as String;
      } catch (_) {}
      yield ChatErrorEvent(code: code, message: body);
      return;
    }

    final lines = resp.stream.transform(utf8.decoder).transform(const LineSplitter());
    yield* parseSseLines(lines);
  }
}

/// Parses SSE `event:`/`data:` line pairs (blank line = dispatch) into typed
/// [ChatEvent]s. Pure — drives directly off a line stream in tests.
Stream<ChatEvent> parseSseLines(Stream<String> lines) async* {
  String? event;
  final data = StringBuffer();

  ChatEvent? flush() {
    if (event == null) return null;
    final ev = _decodeEvent(event!, data.toString());
    event = null;
    data.clear();
    return ev;
  }

  await for (final line in lines) {
    if (line.isEmpty) {
      final ev = flush();
      if (ev != null) yield ev;
      continue;
    }
    if (line.startsWith('event:')) {
      event = line.substring(6).trim();
    } else if (line.startsWith('data:')) {
      data.write(line.substring(5).trim());
    }
  }
  // Flush a trailing event with no terminating blank line.
  final ev = flush();
  if (ev != null) yield ev;
}

ChatEvent? _decodeEvent(String event, String data) {
  Map<String, dynamic> j;
  try {
    final decoded = jsonDecode(data);
    if (decoded is! Map<String, dynamic>) return null;
    j = decoded;
  } catch (_) {
    return null;
  }
  switch (event) {
    case 'text':
      return ChatTextEvent(j['text'] as String? ?? '');
    case 'tool':
      return ChatToolEvent(
        id: j['id'] as String? ?? '',
        name: j['name'] as String? ?? '',
        status: j['status'] as String? ?? '',
        summary: j['summary'] as String? ?? '',
      );
    case 'done':
      return ChatDoneEvent(
        message: j['message'] as String? ?? '',
        stopReason: j['stop_reason'] as String? ?? '',
      );
    case 'error':
      return ChatErrorEvent(
        code: j['code'] as String? ?? 'unknown',
        message: j['message'] as String? ?? '',
      );
    default:
      return null;
  }
}
