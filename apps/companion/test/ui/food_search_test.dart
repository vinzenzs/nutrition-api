import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kazper/state/app_providers.dart';
import 'package:kazper/ui/camera/food_search.dart';

import '../support/fake_repository.dart';

Widget _harness(FakeRepository repo) => ProviderScope(
      overrides: [repositoryProvider.overrideWithValue(repo)],
      child: const MaterialApp(
        home: Scaffold(body: FoodSearchView()),
      ),
    );

void main() {
  testWidgets('tapping a recent food logs it via the product card', (tester) async {
    final repo = FakeRepository()
      ..recentResults = [productFixture(id: 'p1', name: 'Test bar', lastQ: 40)];
    await tester.pumpWidget(_harness(repo));
    await tester.pumpAndSettle();

    expect(find.text('Test bar'), findsOneWidget);
    await tester.tap(find.text('Test bar'));
    await tester.pumpAndSettle();

    // Shared product card is shown; Log commits the meal.
    expect(find.text('Log'), findsOneWidget);
    await tester.tap(find.text('Log'));
    await tester.pumpAndSettle();

    expect(repo.meals, hasLength(1));
    expect(repo.meals.single['product_id'], 'p1');
    expect(repo.meals.single['quantity_g'], 40); // last_logged_quantity_g default
  });

  testWidgets('no-match search offers create that saves the food', (tester) async {
    final repo = FakeRepository()
      ..recentResults = []
      ..searchResults = []
      ..cachedSearch = [];
    await tester.pumpWidget(_harness(repo));
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextField), 'granola');
    await tester.pumpAndSettle(); // debounce + empty results

    expect(find.text('Create "granola"'), findsOneWidget);
    await tester.tap(find.text('Create "granola"'));
    await tester.pumpAndSettle();

    // Quick-create sheet, name pre-filled from the query.
    expect(find.text('Create food'), findsOneWidget);
    await tester.tap(find.text('Save & log'));
    await tester.pumpAndSettle();

    expect(repo.meals, hasLength(1));
    expect(repo.meals.single['name'], 'granola');
    expect(repo.meals.single['save_as_product'], true);
  });
}
