import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../domain/models.dart';
import 'app_providers.dart';

/// Stale-while-revalidate daily summary. On build it returns the cached
/// summary immediately (if present) and kicks off a background revalidation;
/// otherwise it awaits the network. There is no offline banner — staleness is
/// implicit (per the spec).
class TodayNotifier extends AsyncNotifier<DailySummary?> {
  @override
  Future<DailySummary?> build() async {
    final repo = ref.watch(repositoryProvider);
    final date = todayDate();
    final cached = await repo.cachedDailySummary(date);
    if (cached != null) {
      // Render cache now; revalidate without blocking the first frame.
      _revalidate(date);
      return cached;
    }
    return repo.fetchDailySummary(date);
  }

  Future<void> _revalidate(String date) async {
    try {
      final fresh = await ref.read(repositoryProvider).fetchDailySummary(date);
      state = AsyncData(fresh);
    } catch (_) {
      // Keep the stale cache visible — no banner, no error surface.
    }
  }

  /// Pull-to-refresh / post-write refresh.
  Future<void> refresh() async {
    final date = todayDate();
    state = await AsyncValue.guard(
      () => ref.read(repositoryProvider).fetchDailySummary(date),
    );
  }
}

final todayProvider =
    AsyncNotifierProvider<TodayNotifier, DailySummary?>(TodayNotifier.new);

/// Today's hydration total + entries. Plain fetch (cheap, small payload).
final hydrationDailyProvider = FutureProvider<HydrationDaily>((ref) {
  return ref.watch(repositoryProvider).fetchHydrationDaily(todayDate());
});
