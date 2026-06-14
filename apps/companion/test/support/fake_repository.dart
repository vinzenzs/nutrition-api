import 'dart:typed_data';

import 'package:kazper/data/repository.dart';
import 'package:kazper/domain/models.dart';
import 'package:kazper/domain/planning.dart';

/// Behaviour-only fake — no Drift, no Dio. Tests set the public fields to wire
/// responses and read [meals]/[deletedMeals]/[hydration] to assert writes.
class FakeRepository implements Repository {
  DailySummary? cached;
  DailySummary? fresh;
  HydrationDaily? hydrationDaily;
  Goals? goals;
  Product? cachedProductValue;
  Product? lookupValue;
  Object? lookupError;
  PhotoMealResult? photoResult;
  Object? photoError;

  // Food picker wiring. `networkError`, when set, makes the network reads
  // (searchProducts/recentProducts) throw so tests can exercise the offline
  // fallback to the cached* lists.
  List<Product> searchResults = [];
  List<Product> recentResults = [];
  List<Product> cachedRecent = [];
  List<Product> cachedSearch = [];
  Object? networkError;
  final List<String> searchQueries = [];

  final List<Map<String, dynamic>> meals = [];
  final List<String> deletedMeals = [];
  final List<double> hydrationLogs = [];

  /// Meals appended by enqueue* calls, surfaced on the next read so an
  /// optimistic write shows up in Today/Recent (lets an integration test go
  /// scan → log → see it).
  final List<MealEntry> _appended = [];

  @override
  Future<DailySummary?> cachedDailySummary(String date) async => cached;

  @override
  Future<DailySummary> fetchDailySummary(String date) async {
    final base = fresh ?? cached ?? summaryFixture(date: date);
    return DailySummary(
      date: base.date,
      tz: base.tz,
      totals: base.totals,
      entries: [...base.entries, ..._appended],
      adherence: base.adherence,
    );
  }

  @override
  Future<HydrationDaily> fetchHydrationDaily(String date) async =>
      hydrationDaily ??
      HydrationDaily(date: date, tz: 'UTC', totalMl: 0, entries: []);

  @override
  Future<Goals?> fetchGoals() async => goals;

  @override
  Future<Product?> cachedProduct(String id) async => cachedProductValue;

  @override
  Future<Product> lookupProduct(String barcode) async {
    if (lookupError != null) throw lookupError!;
    return lookupValue!;
  }

  @override
  Future<List<Product>> searchProducts(String q) async {
    searchQueries.add(q);
    if (networkError != null) throw networkError!;
    return searchResults;
  }

  @override
  Future<List<Product>> recentProducts({int limit = 50, int offset = 0}) async {
    if (networkError != null) throw networkError!;
    return recentResults;
  }

  @override
  Future<List<Product>> cachedRecentProducts(int limit) async => cachedRecent;

  @override
  Future<List<Product>> cachedSearchProducts(String q) async => cachedSearch;

  @override
  Future<PhotoMealResult> logMealFromPhoto({
    required Uint8List jpegBytes,
    required double quantityG,
    required String mealType,
    required DateTime loggedAt,
  }) async {
    if (photoError != null) throw photoError!;
    return photoResult!;
  }

  @override
  Future<void> enqueueMeal({
    required String productId,
    required double quantityG,
    required String mealType,
    required DateTime loggedAt,
  }) async {
    meals.add({
      'product_id': productId,
      'quantity_g': quantityG,
      'meal_type': mealType,
    });
    _appended.add(mealFixture(
      id: 'logged-${meals.length}',
      name: lookupValue?.name ?? cachedProductValue?.name ?? 'Logged meal',
      mealType: mealType,
    ));
  }

  @override
  Future<void> enqueueFreeformMeal({
    required String name,
    required double quantityG,
    required String mealType,
    required DateTime loggedAt,
    double? kcal,
    double? proteinG,
    double? carbsG,
    double? fatG,
    Map<String, double>? micros,
    bool saveAsProduct = false,
  }) async {
    meals.add({
      'name': name,
      'quantity_g': quantityG,
      'freeform': true,
      'save_as_product': saveAsProduct,
    });
    _appended.add(mealFixture(id: 'freeform-${meals.length}', name: name));
  }

  @override
  Future<void> enqueuePatchMeal(String id,
      {double? quantityG, String? mealType}) async {}

  @override
  Future<void> enqueueDeleteMeal(String id) async => deletedMeals.add(id);

  @override
  Future<void> enqueueHydration(
      {required double quantityMl, DateTime? loggedAt}) async {
    hydrationLogs.add(quantityMl);
  }

  @override
  Future<void> enqueueDeleteHydration(String id) async {}

  // --- planning surfaces (chat) ---
  List<PlannedMeal> plan = [];
  List<ShoppingItem> shopping = [];
  final List<String> eaten = [];
  final List<(String, String)> planStatus = [];
  final List<(String, bool)> shoppingChecked = [];
  final List<String> addedShopping = [];
  int clearCheckedCalls = 0;

  @override
  Future<List<PlannedMeal>> cachedPlan(String date) async => const [];
  @override
  Future<List<PlannedMeal>> fetchPlan(String date) async => plan;
  @override
  Future<List<ShoppingItem>> cachedShopping() async => const [];
  @override
  Future<List<ShoppingItem>> fetchShopping() async => shopping;
  // Writes mutate the fake's server-side state so a subsequent fetch (the
  // provider's reconcile) sees the applied change, like the real backend.
  @override
  Future<void> enqueueMarkEaten(String planId) async {
    eaten.add(planId);
    plan = [for (final p in plan) p.id == planId ? p.copyWith(status: 'eaten') : p];
  }

  @override
  Future<void> enqueuePlanStatus(String planId, String status) async {
    planStatus.add((planId, status));
    plan = [for (final p in plan) p.id == planId ? p.copyWith(status: status) : p];
  }

  @override
  Future<void> enqueueShoppingChecked(String itemId, bool checked) async {
    shoppingChecked.add((itemId, checked));
    shopping = [for (final i in shopping) i.id == itemId ? i.copyWith(checked: checked) : i];
  }

  @override
  Future<void> enqueueAddShoppingItem(String name) async {
    addedShopping.add(name);
    shopping = [...shopping, ShoppingItem(id: 'new-$name', name: name, checked: false)];
  }

  @override
  Future<void> enqueueClearCheckedShopping() async {
    clearCheckedCalls++;
    shopping = shopping.where((i) => !i.checked).toList();
  }

  @override
  Future<void> flush() async {}
}

DailySummary summaryFixture({
  String date = '2026-06-10',
  List<MealEntry> entries = const [],
  Adherence? adherence,
}) {
  return DailySummary(
    date: date,
    tz: 'UTC',
    totals: Totals(kcal: 1800, proteinG: 90, carbsG: 200, fatG: 60),
    entries: entries,
    adherence: adherence,
  );
}

MealEntry mealFixture({
  String id = 'm1',
  String name = 'Oats',
  DateTime? at,
  String? mealType = 'breakfast',
}) {
  return MealEntry(
    id: id,
    loggedAt: at ?? DateTime.utc(2026, 6, 10, 8),
    quantityG: 100,
    mealType: mealType,
    effectiveName: name,
    effectiveNutrimentsPer100g:
        Nutriments(kcal: 350, proteinG: 12, carbsG: 60, fatG: 7),
  );
}

Product productFixture({
  String id = '12345',
  String name = 'Test bar',
  double? lastQ,
  double? serving,
}) =>
    Product(
      id: id,
      name: name,
      source: 'off',
      nutrimentsPer100g:
          Nutriments(kcal: 100, proteinG: 5, carbsG: 10, fatG: 2),
      lastLoggedQuantityG: lastQ,
      servingSizeG: serving,
    );
