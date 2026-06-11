import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/repository.dart';
import '../domain/models.dart';
import 'app_providers.dart';

enum ScanPhase { idle, lookingUp, product, notFound, error, logged }

class ScanState {
  final ScanPhase phase;
  final String? barcode;
  final Product? product;
  final double quantityG;
  final String mealType;
  final String? error;

  const ScanState({
    this.phase = ScanPhase.idle,
    this.barcode,
    this.product,
    this.quantityG = 100,
    this.mealType = 'snack',
    this.error,
  });

  ScanState copyWith({
    ScanPhase? phase,
    String? barcode,
    Product? product,
    double? quantityG,
    String? mealType,
    String? error,
  }) {
    return ScanState(
      phase: phase ?? this.phase,
      barcode: barcode ?? this.barcode,
      product: product ?? this.product,
      quantityG: quantityG ?? this.quantityG,
      mealType: mealType ?? this.mealType,
      error: error,
    );
  }
}

/// Default quantity per spec: last_logged_quantity_g → serving_size_g → 100.
double defaultQuantityFor(Product p) =>
    p.lastLoggedQuantityG ?? p.servingSizeG ?? 100;

class ScanNotifier extends Notifier<ScanState> {
  @override
  ScanState build() => const ScanState();

  Repository get _repo => ref.read(repositoryProvider);

  /// Handles a freshly detected barcode: cache-first, then network lookup.
  /// Re-detections of the same barcode while already showing the product are
  /// ignored so the viewfinder doesn't thrash.
  Future<void> onBarcode(String barcode) async {
    if (state.barcode == barcode &&
        (state.phase == ScanPhase.product ||
            state.phase == ScanPhase.lookingUp)) {
      return;
    }
    state = ScanState(phase: ScanPhase.lookingUp, barcode: barcode);

    final cached = await _repo.cachedProduct(barcode);
    if (cached != null) {
      _showProduct(cached, barcode);
      // Revalidate in the background; ignore failures (offline-friendly).
      _repo.lookupProduct(barcode).then(
            (fresh) {
              if (state.barcode == barcode) _showProduct(fresh, barcode);
            },
            onError: (_) {},
          );
      return;
    }

    try {
      final fresh = await _repo.lookupProduct(barcode);
      if (state.barcode == barcode) _showProduct(fresh, barcode);
    } on ProductNotFound {
      if (state.barcode == barcode) {
        state = state.copyWith(phase: ScanPhase.notFound);
      }
    } catch (e) {
      if (state.barcode == barcode) {
        state = state.copyWith(phase: ScanPhase.error, error: e.toString());
      }
    }
  }

  void _showProduct(Product p, String barcode) {
    state = ScanState(
      phase: ScanPhase.product,
      barcode: barcode,
      product: p,
      quantityG: defaultQuantityFor(p),
      mealType: mealTypeForNow(),
    );
  }

  void setQuantity(double q) => state = state.copyWith(quantityG: q);

  void setMealType(String t) => state = state.copyWith(mealType: t);

  /// Enqueues `POST /meals` for the scanned product and resets to idle so the
  /// viewfinder is ready for the next scan.
  Future<void> log() async {
    final p = state.product;
    if (p == null) return;
    await _repo.enqueueMeal(
      productId: p.id,
      quantityG: state.quantityG,
      mealType: state.mealType,
      loggedAt: DateTime.now(),
    );
    state = state.copyWith(phase: ScanPhase.logged);
  }

  void reset() => state = const ScanState();
}

final scanProvider =
    NotifierProvider<ScanNotifier, ScanState>(ScanNotifier.new);
