import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:package_info_plus/package_info_plus.dart';

import '../../state/app_providers.dart';
import '../../state/pairing_provider.dart';

Future<void> showSettingsSheet(BuildContext context) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (_) => const _SettingsSheet(),
  );
}

class _SettingsSheet extends ConsumerStatefulWidget {
  const _SettingsSheet();

  @override
  ConsumerState<_SettingsSheet> createState() => _SettingsSheetState();
}

class _SettingsSheetState extends ConsumerState<_SettingsSheet> {
  String? _baseUrl;
  String _health = '…';
  String _version = '';
  late int _glassSize;

  @override
  void initState() {
    super.initState();
    _glassSize = ref.read(prefsProvider).glassSizeMl;
    _load();
  }

  Future<void> _load() async {
    final baseUrl = await ref.read(tokenStoreProvider).getBaseUrl();
    final info = await PackageInfo.fromPlatform();
    String health;
    try {
      final resp = await ref.read(apiClientProvider).dio.get<dynamic>(
            '/healthz',
            options: Options(validateStatus: (_) => true),
          );
      health = resp.statusCode == 200 ? 'healthy' : 'unhealthy (${resp.statusCode})';
    } on DioException {
      health = 'unreachable';
    }
    if (!mounted) return;
    setState(() {
      _baseUrl = baseUrl;
      _health = health;
      _version = '${info.version}+${info.buildNumber}';
    });
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        16,
        0,
        16,
        16 + MediaQuery.of(context).viewInsets.bottom,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Settings', style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 12),
          ListTile(
            contentPadding: EdgeInsets.zero,
            leading: const Icon(Icons.dns_outlined),
            title: const Text('Server'),
            subtitle: Text(_baseUrl ?? '—'),
            trailing: Chip(label: Text(_health)),
          ),
          ListTile(
            contentPadding: EdgeInsets.zero,
            leading: const Icon(Icons.local_drink_outlined),
            title: const Text('Glass size'),
            subtitle: Text('$_glassSize ml'),
            trailing: SizedBox(
              width: 160,
              child: Slider(
                min: 100,
                max: 750,
                divisions: 13,
                value: _glassSize.toDouble(),
                label: '$_glassSize ml',
                onChanged: (v) => setState(() => _glassSize = v.round()),
                onChangeEnd: (v) =>
                    ref.read(prefsProvider).setGlassSizeMl(v.round()),
              ),
            ),
          ),
          ListTile(
            contentPadding: EdgeInsets.zero,
            leading: const Icon(Icons.info_outline),
            title: const Text('Version'),
            subtitle: Text(_version.isEmpty ? '—' : _version),
          ),
          const SizedBox(height: 8),
          OutlinedButton.icon(
            icon: const Icon(Icons.logout),
            label: const Text('Unpair'),
            style: OutlinedButton.styleFrom(
              foregroundColor: Theme.of(context).colorScheme.error,
            ),
            onPressed: () async {
              await ref.read(pairingProvider.notifier).unpair();
              if (context.mounted) Navigator.of(context).pop();
            },
          ),
        ],
      ),
    );
  }
}
