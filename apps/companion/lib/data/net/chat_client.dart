import 'dart:convert';

import 'package:http/http.dart' as http;

import '../../domain/chat.dart';
import '../auth/token_store.dart';

/// Streams a chat turn over the backend SSE endpoint. The backend is stateless,
/// so each call POSTs the full client-held transcript and the response is a
/// `text/event-stream` of four event types. Hand-rolled (no SSE package) per
/// design D2 — the parser is pulled out as a pure function for testing.
class ChatClient {
  final TokenStore tokenStore;
  final http.Client _http;

  ChatClient({required this.tokenStore, http.Client? client})
      : _http = client ?? http.Client();

  Stream<ChatEvent> stream(List<ChatMessage> history) async* {
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
      ..body = jsonEncode({
        'messages': [
          for (final m in history)
            {'role': chatRoleName(m.role), 'content': m.content},
        ],
      });

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
