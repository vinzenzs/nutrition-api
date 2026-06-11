import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../domain/planning.dart';
import 'app_providers.dart';

class ShoppingState {
  final List<ShoppingItem> items;
  final bool loading;

  const ShoppingState({this.items = const [], this.loading = false});

  ShoppingState copyWith({List<ShoppingItem>? items, bool? loading}) =>
      ShoppingState(items: items ?? this.items, loading: loading ?? this.loading);

  int get openCount => items.where((i) => !i.checked).length;
}

class ShoppingNotifier extends Notifier<ShoppingState> {
  @override
  ShoppingState build() {
    _load();
    return const ShoppingState(loading: true);
  }

  Future<void> _load() async {
    final repo = ref.read(repositoryProvider);
    final cached = await repo.cachedShopping();
    if (cached.isNotEmpty) {
      state = state.copyWith(items: cached, loading: false);
    }
    try {
      final fresh = await repo.fetchShopping();
      state = state.copyWith(items: fresh, loading: false);
    } catch (_) {
      state = state.copyWith(loading: false);
    }
  }

  Future<void> refresh() => _load();

  /// Optimistic check/uncheck.
  Future<void> setChecked(String id, bool checked) async {
    state = state.copyWith(
      items: [for (final i in state.items) i.id == id ? i.copyWith(checked: checked) : i],
    );
    final repo = ref.read(repositoryProvider);
    await repo.enqueueShoppingChecked(id, checked);
    await repo.flush();
    await _load();
  }

  Future<void> addItem(String name) async {
    final trimmed = name.trim();
    if (trimmed.isEmpty) return;
    final repo = ref.read(repositoryProvider);
    await repo.enqueueAddShoppingItem(trimmed);
    await repo.flush();
    await _load();
  }

  Future<void> clearChecked() async {
    state = state.copyWith(items: state.items.where((i) => !i.checked).toList());
    final repo = ref.read(repositoryProvider);
    await repo.enqueueClearCheckedShopping();
    await repo.flush();
    await _load();
  }
}

final shoppingProvider =
    NotifierProvider<ShoppingNotifier, ShoppingState>(ShoppingNotifier.new);

/// Badge count = open (unchecked) items.
final shoppingOpenCountProvider =
    Provider<int>((ref) => ref.watch(shoppingProvider).openCount);
