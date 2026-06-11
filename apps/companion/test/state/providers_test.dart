import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nutrition_companion/data/repository.dart';
import 'package:nutrition_companion/domain/models.dart';
import 'package:nutrition_companion/state/app_providers.dart';
import 'package:nutrition_companion/state/goals_provider.dart';
import 'package:nutrition_companion/state/recent_provider.dart';
import 'package:nutrition_companion/state/scan_provider.dart';
import 'package:nutrition_companion/state/today_provider.dart';

import '../support/fake_repository.dart';

ProviderContainer _container(FakeRepository repo) {
  final c = ProviderContainer(
    overrides: [repositoryProvider.overrideWithValue(repo)],
  );
  addTearDown(c.dispose);
  return c;
}

void main() {
  group('goalsProvider', () {
    test('passes through null (no goals set)', () async {
      final repo = FakeRepository()..goals = null;
      expect(await _container(repo).read(goalsProvider.future), isNull);
    });

    test('passes through goals', () async {
      final repo = FakeRepository()
        ..goals = Goals({'kcal': GoalRange(min: 2000, max: 2500)});
      final goals = await _container(repo).read(goalsProvider.future);
      expect(goals!.ranges['kcal']!.min, 2000);
    });
  });

  group('todayProvider', () {
    test('fetches from network when no cache', () async {
      final repo = FakeRepository()..fresh = summaryFixture();
      final summary = await _container(repo).read(todayProvider.future);
      expect(summary!.date, '2026-06-10');
    });

    test('returns cache first, then revalidates to fresh', () async {
      final repo = FakeRepository()
        ..cached = summaryFixture()
        ..fresh = summaryFixture();
      final c = _container(repo);
      expect(await c.read(todayProvider.future), isNotNull);
      await Future<void>.delayed(Duration.zero);
      expect(c.read(todayProvider).value, isNotNull);
    });
  });

  group('scanProvider', () {
    test('cached product → product phase with default quantity', () async {
      final repo = FakeRepository()
        ..cachedProductValue = productFixture(lastQ: 42, serving: 30);
      final c = _container(repo);
      await c.read(scanProvider.notifier).onBarcode('12345');
      final s = c.read(scanProvider);
      expect(s.phase, ScanPhase.product);
      expect(s.quantityG, 42); // last_logged_quantity_g wins
    });

    test('falls back to serving size, then 100', () {
      expect(defaultQuantityFor(productFixture(serving: 30)), 30);
      expect(defaultQuantityFor(productFixture()), 100);
    });

    test('404 → notFound phase', () async {
      final repo = FakeRepository()..lookupError = ProductNotFound('99');
      final c = _container(repo);
      await c.read(scanProvider.notifier).onBarcode('99');
      expect(c.read(scanProvider).phase, ScanPhase.notFound);
    });

    test('log enqueues a meal and moves to logged', () async {
      final repo = FakeRepository()
        ..cachedProductValue = productFixture(serving: 30);
      final c = _container(repo);
      final n = c.read(scanProvider.notifier);
      await n.onBarcode('12345');
      await n.log();
      expect(repo.meals, hasLength(1));
      expect(repo.meals.first['quantity_g'], 30);
      expect(c.read(scanProvider).phase, ScanPhase.logged);
    });
  });

  group('recentProvider', () {
    test('merges meals + hydration newest-first', () async {
      final meal = mealFixture(at: DateTime.utc(2026, 6, 10, 8));
      final repo = FakeRepository()
        ..fresh = summaryFixture(entries: [meal])
        ..hydrationDaily = HydrationDaily(
          date: '2026-06-10',
          tz: 'UTC',
          totalMl: 250,
          entries: [
            HydrationEntry(
                id: 'h1',
                loggedAt: DateTime.utc(2026, 6, 10, 12),
                quantityMl: 250),
          ],
        );
      final items = await _container(repo).read(recentProvider.future);
      expect(items, hasLength(2));
      expect(items.first, isA<RecentHydration>()); // 12:00 newest
      expect(items.last, isA<RecentMeal>());
    });
  });
}
