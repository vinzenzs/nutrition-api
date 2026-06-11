import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../domain/planning.dart';
import 'app_providers.dart';

class PlanState {
  final List<PlannedMeal> entries;
  final bool loading;

  /// Ids with an eaten/skip write still in flight — their actions are disabled
  /// ("syncing…") so an offline double-tap can't queue a guaranteed 409.
  final Set<String> pending;

  const PlanState({
    this.entries = const [],
    this.loading = false,
    this.pending = const {},
  });

  PlanState copyWith({
    List<PlannedMeal>? entries,
    bool? loading,
    Set<String>? pending,
  }) =>
      PlanState(
        entries: entries ?? this.entries,
        loading: loading ?? this.loading,
        pending: pending ?? this.pending,
      );
}

class PlanNotifier extends Notifier<PlanState> {
  @override
  PlanState build() {
    _load();
    return const PlanState(loading: true);
  }

  Future<void> _load() async {
    final repo = ref.read(repositoryProvider);
    final date = todayDate();
    final cached = await repo.cachedPlan(date);
    if (cached.isNotEmpty) {
      state = state.copyWith(entries: cached, loading: false);
    }
    try {
      final fresh = await repo.fetchPlan(date);
      // Drop pending markers the server has caught up on.
      final stillPending = state.pending
          .where((id) => fresh.any((e) => e.id == id && e.status == 'planned'))
          .toSet();
      state = state.copyWith(entries: fresh, loading: false, pending: stillPending);
    } catch (_) {
      state = state.copyWith(loading: false);
    }
  }

  Future<void> refresh() => _load();

  Future<void> markEaten(String id) => _transition(id, 'eaten', eaten: true);
  Future<void> skip(String id) => _transition(id, 'skipped', eaten: false);

  Future<void> _transition(String id, String status, {required bool eaten}) async {
    if (state.pending.contains(id)) return;
    final entry = state.entries.where((e) => e.id == id).firstOrNull;
    if (entry == null || entry.status != 'planned') return;

    // Optimistic flip + mark pending.
    state = state.copyWith(
      entries: [for (final e in state.entries) e.id == id ? e.copyWith(status: status) : e],
      pending: {...state.pending, id},
    );

    final repo = ref.read(repositoryProvider);
    if (eaten) {
      await repo.enqueueMarkEaten(id);
    } else {
      await repo.enqueuePlanStatus(id, status);
    }
    await repo.flush();
    // Reconcile against server truth (reverts the flip if the write failed).
    await _load();
  }
}

final planProvider = NotifierProvider<PlanNotifier, PlanState>(PlanNotifier.new);

/// Convenience: today's plan entries (for the card).
final todayPlanProvider = Provider<List<PlannedMeal>>(
  (ref) => ref.watch(planProvider).entries,
);
