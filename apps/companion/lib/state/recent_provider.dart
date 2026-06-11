import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../domain/models.dart';
import 'app_providers.dart';

/// A single Recent-screen row: either a logged meal or a hydration entry,
/// unified so the list can sort both by `logged_at` descending.
sealed class RecentItem {
  DateTime get loggedAt;
}

class RecentMeal extends RecentItem {
  final MealEntry meal;
  RecentMeal(this.meal);
  @override
  DateTime get loggedAt => meal.loggedAt;
}

class RecentHydration extends RecentItem {
  final HydrationEntry entry;
  RecentHydration(this.entry);
  @override
  DateTime get loggedAt => entry.loggedAt;
}

/// Today's meals (from the daily summary) merged with today's hydration
/// entries, newest first. Writes go through the async outbox, so this can't be
/// re-fetched immediately after a log (the GET would race the POST). Instead
/// the shell calls [refresh] when the Recent tab is entered, by which point the
/// outbox has flushed; [refresh] uses AsyncValue.guard so the list doesn't
/// flash a spinner on every reconcile.
class RecentNotifier extends AsyncNotifier<List<RecentItem>> {
  @override
  Future<List<RecentItem>> build() => _load();

  Future<List<RecentItem>> _load() async {
    final repo = ref.read(repositoryProvider);
    final date = todayDate();
    final summary = await repo.fetchDailySummary(date);
    final hydration = await repo.fetchHydrationDaily(date);
    return <RecentItem>[
      ...summary.entries.map(RecentMeal.new),
      ...hydration.entries.map(RecentHydration.new),
    ]..sort((a, b) => b.loggedAt.compareTo(a.loggedAt));
  }

  /// Re-fetch and reconcile, without dropping to a loading state.
  Future<void> refresh() async {
    state = await AsyncValue.guard(_load);
  }
}

final recentProvider =
    AsyncNotifierProvider<RecentNotifier, List<RecentItem>>(RecentNotifier.new);
