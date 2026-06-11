import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:mobile_scanner/mobile_scanner.dart';

import '../../state/pairing_provider.dart';

/// First-run screen. Scans the QR printed by `task dev:pair`, whose payload is
/// `{"base_url": "<url>", "token": "<bearer>"}`, validates it, and pairs.
class PairPage extends ConsumerStatefulWidget {
  const PairPage({super.key});

  @override
  ConsumerState<PairPage> createState() => _PairPageState();
}

class _PairPageState extends ConsumerState<PairPage> {
  final _controller = MobileScannerController();
  String? _error;
  bool _handling = false;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _onDetect(BarcodeCapture capture) async {
    if (_handling) return;
    final raw = capture.barcodes.isNotEmpty ? capture.barcodes.first.rawValue : null;
    if (raw == null) return;
    await _pairFromRaw(raw, malformedMessage: 'That QR code is not a valid pairing payload. Try again.');
  }

  /// Parses a raw pairing payload and pairs. Shared by the QR scanner and the
  /// paste fallback (useful on an emulator, where scanning a QR is impractical).
  Future<void> _pairFromRaw(String raw, {required String malformedMessage}) async {
    if (_handling) return;
    _handling = true;

    final parsed = _parse(raw);
    if (parsed == null) {
      setState(() {
        _error = malformedMessage;
        _handling = false;
      });
      return;
    }

    try {
      await ref.read(pairingProvider.notifier).pair(
            baseUrl: parsed.$1,
            token: parsed.$2,
          );
      // The root widget swaps to the home shell when pairing flips to true.
    } catch (e) {
      setState(() {
        _error = 'Pairing failed: $e';
        _handling = false;
      });
    }
  }

  /// Prompts for a pasted `{"base_url":...,"token":...}` payload, then pairs.
  Future<void> _showPasteDialog() async {
    final controller = TextEditingController();
    final raw = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Paste pairing payload'),
        content: TextField(
          controller: controller,
          autofocus: true,
          minLines: 3,
          maxLines: 6,
          decoration: const InputDecoration(
            hintText: '{"base_url":"http://10.0.2.2:8080","token":"..."}',
            border: OutlineInputBorder(),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, controller.text.trim()),
            child: const Text('Pair'),
          ),
        ],
      ),
    );
    if (raw == null || raw.isEmpty) return;
    await _pairFromRaw(raw, malformedMessage: 'That payload is not valid JSON with base_url + token.');
  }

  /// Returns (baseUrl, token) or null if the payload is malformed.
  (String, String)? _parse(String raw) {
    try {
      final json = jsonDecode(raw);
      if (json is! Map) return null;
      final baseUrl = json['base_url'];
      final token = json['token'];
      if (baseUrl is! String || token is! String) return null;
      if (token.isEmpty) return null;
      final uri = Uri.tryParse(baseUrl);
      if (uri == null || !uri.isAbsolute) return null;
      return (baseUrl, token);
    } catch (_) {
      return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Pair with your backend')),
      body: Column(
        children: [
          Expanded(
            child: Stack(
              fit: StackFit.expand,
              children: [
                MobileScanner(controller: _controller, onDetect: _onDetect),
                const _ScanOverlay(),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              children: [
                Text(
                  'Run  task dev:pair  on your backend and point the camera at '
                  'the QR code in the terminal.',
                  textAlign: TextAlign.center,
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
                const SizedBox(height: 4),
                TextButton.icon(
                  onPressed: _handling ? null : _showPasteDialog,
                  icon: const Icon(Icons.content_paste),
                  label: const Text('Paste payload instead'),
                ),
                if (_error != null) ...[
                  const SizedBox(height: 12),
                  Text(
                    _error!,
                    textAlign: TextAlign.center,
                    style: TextStyle(color: Theme.of(context).colorScheme.error),
                  ),
                  const SizedBox(height: 8),
                  FilledButton.tonal(
                    onPressed: () => setState(() => _error = null),
                    child: const Text('Try again'),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _ScanOverlay extends StatelessWidget {
  const _ScanOverlay();

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Container(
        width: 220,
        height: 220,
        decoration: BoxDecoration(
          border: Border.all(color: Colors.white70, width: 3),
          borderRadius: BorderRadius.circular(16),
        ),
      ),
    );
  }
}
