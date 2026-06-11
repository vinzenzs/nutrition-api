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
/// entries, newest first. Combines the two API calls the spec calls out.
final recentProvider = FutureProvider<List<RecentItem>>((ref) async {
  final repo = ref.watch(repositoryProvider);
  final date = todayDate();
  final summary = await repo.fetchDailySummary(date);
  final hydration = await repo.fetchHydrationDaily(date);

  final items = <RecentItem>[
    ...summary.entries.map(RecentMeal.new),
    ...hydration.entries.map(RecentHydration.new),
  ]..sort((a, b) => b.loggedAt.compareTo(a.loggedAt));
  return items;
});
