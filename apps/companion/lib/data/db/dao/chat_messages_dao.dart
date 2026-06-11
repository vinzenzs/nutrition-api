import 'package:drift/drift.dart';

import '../app_database.dart';

part 'chat_messages_dao.g.dart';

@DriftAccessor(tables: [ChatMessages])
class ChatMessagesDao extends DatabaseAccessor<AppDatabase>
    with _$ChatMessagesDaoMixin {
  ChatMessagesDao(super.db);

  Future<void> insertMessage({
    required String id,
    required String conversationId,
    required String role,
    required String content,
    required DateTime createdAt,
  }) {
    return into(chatMessages).insert(ChatMessagesCompanion.insert(
      id: id,
      conversationId: conversationId,
      role: role,
      content: content,
      createdAt: createdAt,
    ));
  }

  Future<List<ChatMessage>> forConversation(String conversationId) {
    return (select(chatMessages)
          ..where((m) => m.conversationId.equals(conversationId))
          ..orderBy([(m) => OrderingTerm.asc(m.createdAt)]))
        .get();
  }

  /// Keep only the [keep] most-recently-active conversations; drop older ones.
  Future<void> pruneConversations({int keep = 20}) async {
    final rows = await customSelect(
      'SELECT conversation_id, MAX(created_at) AS m FROM chat_messages '
      'GROUP BY conversation_id ORDER BY m DESC',
    ).get();
    if (rows.length <= keep) return;
    final stale = rows.skip(keep).map((r) => r.read<String>('conversation_id')).toList();
    await (delete(chatMessages)..where((m) => m.conversationId.isIn(stale))).go();
  }
}
