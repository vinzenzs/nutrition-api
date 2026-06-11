import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app_providers.dart';

/// Whether the app holds a bearer token. The root widget watches this to pick
/// between the pairing screen and the home shell. `null` while the initial
/// secure-storage read is in flight.
class PairingNotifier extends AsyncNotifier<bool> {
  @override
  Future<bool> build() async {
    final token = await ref.watch(tokenStoreProvider).getToken();
    return token != null && token.isNotEmpty;
  }

  /// Persists the scanned pairing payload and flips the app into the home shell.
  Future<void> pair({required String baseUrl, required String token}) async {
    await ref.read(tokenStoreProvider).pair(baseUrl: baseUrl, token: token);
    state = const AsyncData(true);
  }

  /// Clears the token and returns to the pairing screen.
  Future<void> unpair() async {
    await ref.read(tokenStoreProvider).clear();
    state = const AsyncData(false);
  }
}

final pairingProvider =
    AsyncNotifierProvider<PairingNotifier, bool>(PairingNotifier.new);
