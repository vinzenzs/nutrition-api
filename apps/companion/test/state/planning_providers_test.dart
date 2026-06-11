import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nutrition_companion/domain/planning.dart';
import 'package:nutrition_companion/state/app_providers.dart';
import 'package:nutrition_companion/state/plan_provider.dart';
import 'package:nutrition_companion/state/shopping_provider.dart';

import '../support/fake_repository.dart';

ProviderContainer _container(FakeRepository repo) {
  final c = ProviderContainer(
    overrides: [repositoryProvider.overrideWithValue(repo)],
  );
  addTearDown(c.dispose);
  return c;
}

PlannedMeal _meal(String id, {String status = 'planned'}) => PlannedMeal(
      id: id,
      planDate: '2026-06-12',
      slot: 'dinner',
      status: status,
      productName: 'Lasagne',
      quantityG: 450,
    );

void main() {
  group('planProvider', () {
    test('loads entries from the server', () async {
      final repo = FakeRepository()..plan = [_meal('p1')];
      final c = _container(repo);
      await c.read(planProvider.notifier).refresh();
      expect(c.read(planProvider).entries.single.id, 'p1');
    });

    test('markEaten enqueues and reconciles to eaten', () async {
      final repo = FakeRepository()..plan = [_meal('p1')];
      final c = _container(repo);
      await c.read(planProvider.notifier).refresh();
      await c.read(planProvider.notifier).markEaten('p1');
      expect(repo.eaten, ['p1']);
      expect(c.read(planProvider).entries.single.status, 'eaten');
    });

    test('markEaten is a no-op on an already-eaten entry', () async {
      final repo = FakeRepository()..plan = [_meal('p1', status: 'eaten')];
      final c = _container(repo);
      await c.read(planProvider.notifier).refresh();
      await c.read(planProvider.notifier).markEaten('p1');
      expect(repo.eaten, isEmpty, reason: 'no eaten write for a non-planned entry');
    });

    test('skip enqueues a skipped status', () async {
      final repo = FakeRepository()..plan = [_meal('p1')];
      final c = _container(repo);
      await c.read(planProvider.notifier).refresh();
      await c.read(planProvider.notifier).skip('p1');
      expect(repo.planStatus, [('p1', 'skipped')]);
      expect(c.read(planProvider).entries.single.status, 'skipped');
    });
  });

  group('shoppingProvider', () {
    ShoppingItem item(String id, {bool checked = false}) =>
        ShoppingItem(id: id, name: id, checked: checked);

    test('setChecked optimistically checks and reconciles', () async {
      final repo = FakeRepository()..shopping = [item('a')];
      final c = _container(repo);
      await c.read(shoppingProvider.notifier).setChecked('a', true);
      expect(repo.shoppingChecked, [('a', true)]);
      expect(c.read(shoppingProvider).items.single.checked, isTrue);
    });

    test('open count = unchecked items', () async {
      final repo = FakeRepository()
        ..shopping = [item('a'), item('b', checked: true), item('c')];
      final c = _container(repo);
      await c.read(shoppingProvider.notifier).refresh();
      expect(c.read(shoppingOpenCountProvider), 2);
    });

    test('clearChecked removes bought items', () async {
      final repo = FakeRepository()
        ..shopping = [item('a', checked: true), item('b')];
      final c = _container(repo);
      await c.read(shoppingProvider.notifier).clearChecked();
      expect(repo.clearCheckedCalls, 1);
      expect(c.read(shoppingProvider).items.map((i) => i.id), ['b']);
    });

    test('addItem enqueues and appears', () async {
      final repo = FakeRepository()..shopping = [];
      final c = _container(repo);
      await c.read(shoppingProvider.notifier).addItem('Zwiebeln');
      expect(repo.addedShopping, ['Zwiebeln']);
      expect(c.read(shoppingProvider).items.any((i) => i.name == 'Zwiebeln'), isTrue);
    });
  });
}
