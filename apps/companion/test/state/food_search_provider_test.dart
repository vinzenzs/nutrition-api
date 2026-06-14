import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kazper/domain/models.dart';
import 'package:kazper/state/app_providers.dart';
import 'package:kazper/state/food_search_provider.dart';

import '../support/fake_repository.dart';

ProviderContainer _container(FakeRepository repo) {
  final c = ProviderContainer(
    overrides: [repositoryProvider.overrideWithValue(repo)],
  );
  addTearDown(c.dispose);
  return c;
}

/// Polls the provider until its results are data satisfying [until] (defaults
/// to any data). [until] lets a test wait past a stale paint for the value it
/// expects (e.g. the debounced search result, not the initial recent list).
Future<List<Product>> _settle(
  ProviderContainer c, {
  bool Function(List<Product>)? until,
  Duration timeout = const Duration(seconds: 2),
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    await Future<void>.delayed(const Duration(milliseconds: 20));
    final r = c.read(foodSearchProvider).results;
    if (r is AsyncData<List<Product>> &&
        (until == null || until(r.value))) {
      return r.value;
    }
  }
  throw StateError('results never settled: ${c.read(foodSearchProvider).results}');
}

void main() {
  test('empty query resolves to recent products', () async {
    final repo = FakeRepository()
      ..recentResults = [productFixture(id: 'r1', name: 'Recent food')];
    final c = _container(repo);
    c.read(foodSearchProvider); // instantiate → kicks off the initial load

    final foods = await _settle(c);
    expect(foods.single.name, 'Recent food');
  });

  test('non-empty query debounces then searches', () async {
    final repo = FakeRepository()
      ..recentResults = []
      ..searchResults = [productFixture(id: 's1', name: 'Yogurt')];
    final c = _container(repo);
    c.read(foodSearchProvider);
    await _settle(c); // initial recent (empty)

    c.read(foodSearchProvider.notifier).setQuery('yog');
    final foods = await _settle(c, until: (f) => f.any((p) => p.name == 'Yogurt'));
    expect(foods.single.name, 'Yogurt');
    expect(repo.searchQueries, contains('yog'));
  });

  test('offline (network error) falls back to the cache without erroring',
      () async {
    final repo = FakeRepository()
      ..networkError = DioException(requestOptions: RequestOptions(path: '/products'))
      ..cachedRecent = [productFixture(id: 'c1', name: 'Cached food')];
    final c = _container(repo);
    c.read(foodSearchProvider);

    final foods = await _settle(c);
    expect(foods.single.name, 'Cached food');
    expect(c.read(foodSearchProvider).results, isA<AsyncData<List<Product>>>());
  });
}
