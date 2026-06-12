import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../domain/chat.dart';
import '../../state/app_providers.dart';
import '../../state/chat_provider.dart';
import 'sessions_page.dart';

/// The v2 Chat surface — streamed assistant replies, tool-activity chips, and
/// tappable recipe links. Online-only: when offline the composer disables with
/// an inline notice (the lone scoped exception to the no-offline-banner rule).
class ChatPage extends ConsumerStatefulWidget {
  const ChatPage({super.key});

  @override
  ConsumerState<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends ConsumerState<ChatPage> {
  final _composer = TextEditingController();
  final _scroll = ScrollController();

  @override
  void dispose() {
    _composer.dispose();
    _scroll.dispose();
    super.dispose();
  }

  void _send() {
    final text = _composer.text;
    if (text.trim().isEmpty) return;
    _composer.clear();
    ref.read(chatProvider.notifier).send(text);
    _scrollToBottom();
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scroll.hasClients) {
        _scroll.animateTo(_scroll.position.maxScrollExtent,
            duration: const Duration(milliseconds: 200), curve: Curves.easeOut);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final chat = ref.watch(chatProvider);
    final online = ref.watch(onlineProvider).valueOrNull ?? true;
    final notConfigured = chat.error == 'chat_unavailable';

    // Auto-scroll as the streaming bubble grows.
    if (chat.streaming) _scrollToBottom();

    return Scaffold(
      appBar: AppBar(
        title: const Text('Chat'),
        actions: [
          IconButton(
            tooltip: 'History',
            icon: const Icon(Icons.history),
            onPressed: chat.streaming
                ? null
                : () => Navigator.of(context).push(
                      MaterialPageRoute<void>(
                          builder: (_) => const SessionsPage()),
                    ),
          ),
          IconButton(
            tooltip: 'New chat',
            icon: const Icon(Icons.restart_alt),
            onPressed: chat.streaming
                ? null
                : () => ref.read(chatProvider.notifier).newChat(),
          ),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: notConfigured
                ? const _NotConfigured()
                : _Transcript(chat: chat, scroll: _scroll),
          ),
          if (!notConfigured)
            _Composer(
              controller: _composer,
              online: online,
              streaming: chat.streaming,
              onSend: _send,
            ),
        ],
      ),
    );
  }
}

class _Transcript extends StatelessWidget {
  final ChatState chat;
  final ScrollController scroll;
  const _Transcript({required this.chat, required this.scroll});

  @override
  Widget build(BuildContext context) {
    if (chat.messages.isEmpty && !chat.streaming) {
      return const Center(
        child: Padding(
          padding: EdgeInsets.all(32),
          child: Text(
            'Ask what to eat today or the next few days, plan meals, and build a shopping list.',
            textAlign: TextAlign.center,
          ),
        ),
      );
    }
    final itemCount = chat.messages.length + (chat.streaming ? 1 : 0);
    return ListView.builder(
      controller: scroll,
      padding: const EdgeInsets.all(12),
      itemCount: itemCount,
      itemBuilder: (context, i) {
        if (i < chat.messages.length) {
          return _Bubble(message: chat.messages[i]);
        }
        // Streaming assistant bubble + tool chips + (error retry below).
        return _StreamingBubble(text: chat.streamingText ?? '', tools: chat.tools);
      },
    );
  }
}

class _Bubble extends StatelessWidget {
  final ChatMessage message;
  const _Bubble({required this.message});

  @override
  Widget build(BuildContext context) {
    final isUser = message.role == ChatRole.user;
    final scheme = Theme.of(context).colorScheme;
    return Align(
      alignment: isUser ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        margin: const EdgeInsets.symmetric(vertical: 4),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        constraints: BoxConstraints(maxWidth: MediaQuery.of(context).size.width * 0.82),
        decoration: BoxDecoration(
          color: isUser ? scheme.primaryContainer : scheme.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(16),
        ),
        child: isUser
            ? Text(message.content)
            : MarkdownBody(
                data: message.content,
                onTapLink: (text, href, title) {
                  if (href != null) launchUrl(Uri.parse(href), mode: LaunchMode.externalApplication);
                },
              ),
      ),
    );
  }
}

class _StreamingBubble extends StatelessWidget {
  final String text;
  final List<ChatToolEvent> tools;
  const _StreamingBubble({required this.text, required this.tools});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Align(
      alignment: Alignment.centerLeft,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (tools.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(bottom: 4),
              child: Wrap(
                spacing: 6,
                runSpacing: 4,
                children: [for (final t in tools) _ToolChip(tool: t)],
              ),
            ),
          Container(
            margin: const EdgeInsets.symmetric(vertical: 4),
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
            decoration: BoxDecoration(
              color: scheme.surfaceContainerHighest,
              borderRadius: BorderRadius.circular(16),
            ),
            child: text.isEmpty
                ? const SizedBox(
                    width: 18, height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2))
                : MarkdownBody(data: text),
          ),
        ],
      ),
    );
  }
}

class _ToolChip extends StatelessWidget {
  final ChatToolEvent tool;
  const _ToolChip({required this.tool});

  @override
  Widget build(BuildContext context) {
    final error = tool.isError;
    final scheme = Theme.of(context).colorScheme;
    return Chip(
      visualDensity: VisualDensity.compact,
      backgroundColor: error ? scheme.errorContainer : scheme.secondaryContainer,
      avatar: Icon(
        error ? Icons.error_outline
            : tool.status == 'ok' ? Icons.check
            : Icons.bolt,
        size: 16,
        color: error ? scheme.onErrorContainer : null,
      ),
      label: Text(
        // Label by the tool name (status is the avatar icon); append the
        // summary only on error, where it adds information.
        error && tool.summary.isNotEmpty
            ? '${_humanize(tool.name)} — ${tool.summary}'
            : _humanize(tool.name),
        style: TextStyle(fontSize: 12, color: error ? scheme.onErrorContainer : null),
      ),
    );
  }

  // Turns a tool name like "get_daily_context" into "get daily context".
  static String _humanize(String name) => name.replaceAll('_', ' ');
}

class _Composer extends ConsumerWidget {
  final TextEditingController controller;
  final bool online;
  final bool streaming;
  final VoidCallback onSend;

  const _Composer({
    required this.controller,
    required this.online,
    required this.streaming,
    required this.onSend,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final error = ref.watch(chatProvider).error;
    final disabled = !online || streaming;
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(12, 4, 12, 8),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (error != null && error != 'chat_unavailable')
              Padding(
                padding: const EdgeInsets.only(bottom: 6),
                child: Row(
                  children: [
                    Expanded(
                      child: Text('That turn didn\'t complete.',
                          style: TextStyle(color: Theme.of(context).colorScheme.error)),
                    ),
                    TextButton(
                      onPressed: streaming ? null : () => ref.read(chatProvider.notifier).retry(),
                      child: const Text('Retry'),
                    ),
                  ],
                ),
              ),
            if (!online)
              const Padding(
                padding: EdgeInsets.only(bottom: 6),
                child: Text('Chat needs a connection',
                    style: TextStyle(fontStyle: FontStyle.italic)),
              ),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: controller,
                    enabled: !disabled,
                    minLines: 1,
                    maxLines: 5,
                    textInputAction: TextInputAction.send,
                    onSubmitted: disabled ? null : (_) => onSend(),
                    decoration: InputDecoration(
                      hintText: online ? 'Ask about meals…' : 'Offline',
                      border: const OutlineInputBorder(),
                      isDense: true,
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                IconButton.filled(
                  onPressed: disabled ? null : onSend,
                  icon: const Icon(Icons.send),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _NotConfigured extends StatelessWidget {
  const _NotConfigured();

  @override
  Widget build(BuildContext context) {
    return const Center(
      child: Padding(
        padding: EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.chat_bubble_outline, size: 56),
            SizedBox(height: 16),
            Text(
              "Chat isn't configured on this server.",
              textAlign: TextAlign.center,
            ),
            SizedBox(height: 8),
            Text(
              'Set ANTHROPIC_API_KEY on the backend to enable it.',
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 12),
            ),
          ],
        ),
      ),
    );
  }
}
