import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kazper/data/prefs.dart';
import 'package:kazper/domain/models.dart';
import 'package:kazper/state/app_providers.dart';
import 'package:kazper/ui/recent/recent_page.dart';
import 'package:kazper/ui/today/adherence_ring.dart';
import 'package:kazper/ui/today/today_page.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../support/fake_repository.dart';

Future<Widget> _harness(FakeRepository repo, Widget child) async {
  SharedPreferences.setMockInitialValues({});
  final prefs = await Prefs.open();
  return ProviderScope(
    overrides: [
      repositoryProvider.overrideWithValue(repo),
      prefsProvider.overrideWithValue(prefs),
    ],
    child: MaterialApp(home: child),
  );
}

void main() {
  testWidgets('Today renders adherence rings when goals are set',
      (tester) async {
    final adherence = Adherence({
      'kcal': AdherenceRow(status: 'on', actual: 1800, targetMin: 1700, targetMax: 2000),
      'protein_g': AdherenceRow(status: 'under', actual: 90, targetMin: 120, targetMax: 140),
    });
    final repo = FakeRepository()
      ..fresh = summaryFixture(entries: [mealFixture()], adherence: adherence);
    await tester.pumpWidget(await _harness(repo, const TodayPage()));
    await tester.pumpAndSettle();

    expect(find.byType(AdherenceRing), findsWidgets);
    expect(find.text('Recent meals'), findsOneWidget);
    expect(find.text('Oats'), findsOneWidget);
  });

  testWidgets('Today shows raw totals + hint when no goals are set',
      (tester) async {
    final repo = FakeRepository()..fresh = summaryFixture();
    await tester.pumpWidget(await _harness(repo, const TodayPage()));
    await tester.pumpAndSettle();

    expect(find.byType(AdherenceRing), findsNothing);
    expect(find.text('Set goals via the assistant'), findsOneWidget);
  });

  testWidgets('Recent renders meals and hydration, opens a meal sheet',
      (tester) async {
    final repo = FakeRepository()
      ..fresh = summaryFixture(entries: [mealFixture(name: 'Banana')])
      ..hydrationDaily = HydrationDaily(
        date: '2026-06-10',
        tz: 'UTC',
        totalMl: 250,
        entries: [
          HydrationEntry(
              id: 'h1', loggedAt: DateTime.utc(2026, 6, 10, 9), quantityMl: 250),
        ],
      );
    await tester.pumpWidget(await _harness(repo, const RecentPage()));
    await tester.pumpAndSettle();

    expect(find.text('Banana'), findsOneWidget);
    expect(find.textContaining('250 ml'), findsOneWidget);

    await tester.tap(find.text('Banana'));
    await tester.pumpAndSettle();
    // Meal edit sheet opened with Save/Delete affordances.
    expect(find.text('Save'), findsOneWidget);
    expect(find.text('Delete'), findsOneWidget);
  });
}
