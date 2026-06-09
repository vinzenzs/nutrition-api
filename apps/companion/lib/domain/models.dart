// Hand-rolled JSON models mirroring the backend's REST responses.
//
// Conventions:
//   - Fields the backend tags `,omitempty` arrive as `null` here.
//   - Nutriments-per-100g and totals carry a fixed set of macros + an
//     open-ended micro set (iron, calcium, vitamin D/B12/C, mg, K, Zn).
//     Future micros land on the backend without an app upgrade by going
//     through the [Nutriments.micros] / [Totals.micros] maps.
//   - All money/quantity numbers are doubles to match Go's float64 JSON
//     serialization.
//   - Timestamps are parsed to UTC DateTime; the backend always sends RFC 3339.

double? _asDouble(dynamic v) => v == null ? null : (v as num).toDouble();

class Nutriments {
  final double kcal;
  final double proteinG;
  final double carbsG;
  final double fatG;
  final double fiberG;
  final double sugarG;
  final double saltG;
  final Map<String, double> micros;

  Nutriments({
    required this.kcal,
    required this.proteinG,
    required this.carbsG,
    required this.fatG,
    this.fiberG = 0,
    this.sugarG = 0,
    this.saltG = 0,
    this.micros = const {},
  });

  factory Nutriments.fromJson(Map<String, dynamic> json) {
    const knownMicros = [
      'iron_mg',
      'calcium_mg',
      'vitamin_d_mcg',
      'vitamin_b12_mcg',
      'vitamin_c_mg',
      'magnesium_mg',
      'potassium_mg',
      'zinc_mg',
    ];
    final micros = <String, double>{};
    for (final k in knownMicros) {
      final v = _asDouble(json[k]);
      if (v != null) micros[k] = v;
    }
    return Nutriments(
      kcal: _asDouble(json['kcal']) ?? 0,
      proteinG: _asDouble(json['protein_g']) ?? 0,
      carbsG: _asDouble(json['carbs_g']) ?? 0,
      fatG: _asDouble(json['fat_g']) ?? 0,
      fiberG: _asDouble(json['fiber_g']) ?? 0,
      sugarG: _asDouble(json['sugar_g']) ?? 0,
      saltG: _asDouble(json['salt_g']) ?? 0,
      micros: micros,
    );
  }
}

class Product {
  final String id;
  final String name;
  final String? brand;
  final String source;
  final Nutriments nutrimentsPer100g;
  final double? servingSizeG;
  final double? lastLoggedQuantityG;

  Product({
    required this.id,
    required this.name,
    required this.source,
    required this.nutrimentsPer100g,
    this.brand,
    this.servingSizeG,
    this.lastLoggedQuantityG,
  });

  factory Product.fromJson(Map<String, dynamic> json) => Product(
        id: json['id'] as String,
        name: json['name'] as String,
        brand: json['brand'] as String?,
        source: json['source'] as String,
        nutrimentsPer100g: Nutriments.fromJson(
            (json['nutriments_per_100g'] as Map?)?.cast<String, dynamic>() ??
                const <String, dynamic>{}),
        servingSizeG: _asDouble(json['serving_size_g']),
        lastLoggedQuantityG: _asDouble(json['last_logged_quantity_g']),
      );
}

class MealEntry {
  final String id;
  final String? productId;
  final DateTime loggedAt;
  final double quantityG;
  final String? mealType;
  final String? note;
  final String? workoutId;
  final String effectiveName;
  final Nutriments effectiveNutrimentsPer100g;

  MealEntry({
    required this.id,
    required this.loggedAt,
    required this.quantityG,
    required this.effectiveName,
    required this.effectiveNutrimentsPer100g,
    this.productId,
    this.mealType,
    this.note,
    this.workoutId,
  });

  factory MealEntry.fromJson(Map<String, dynamic> json) => MealEntry(
        id: json['id'] as String,
        productId: json['product_id'] as String?,
        loggedAt: DateTime.parse(json['logged_at'] as String),
        quantityG: _asDouble(json['quantity_g']) ?? 0,
        mealType: json['meal_type'] as String?,
        note: json['note'] as String?,
        workoutId: json['workout_id'] as String?,
        effectiveName: json['effective_name'] as String,
        effectiveNutrimentsPer100g: Nutriments.fromJson(
            (json['effective_nutriments_per_100g'] as Map)
                .cast<String, dynamic>()),
      );
}

class HydrationEntry {
  final String id;
  final DateTime loggedAt;
  final double quantityMl;
  final String? note;
  final String? workoutId;

  HydrationEntry({
    required this.id,
    required this.loggedAt,
    required this.quantityMl,
    this.note,
    this.workoutId,
  });

  factory HydrationEntry.fromJson(Map<String, dynamic> json) => HydrationEntry(
        id: json['id'] as String,
        loggedAt: DateTime.parse(json['logged_at'] as String),
        quantityMl: _asDouble(json['quantity_ml']) ?? 0,
        note: json['note'] as String?,
        workoutId: json['workout_id'] as String?,
      );
}

class Totals {
  final double kcal;
  final double proteinG;
  final double carbsG;
  final double fatG;
  final double fiberG;
  final double sugarG;
  final double saltG;
  final Map<String, double> micros;

  Totals({
    required this.kcal,
    required this.proteinG,
    required this.carbsG,
    required this.fatG,
    this.fiberG = 0,
    this.sugarG = 0,
    this.saltG = 0,
    this.micros = const {},
  });

  factory Totals.fromJson(Map<String, dynamic> json) {
    final n = Nutriments.fromJson(json);
    return Totals(
      kcal: n.kcal,
      proteinG: n.proteinG,
      carbsG: n.carbsG,
      fatG: n.fatG,
      fiberG: n.fiberG,
      sugarG: n.sugarG,
      saltG: n.saltG,
      micros: n.micros,
    );
  }
}

/// One row of a daily-summary "adherence" block. The backend sends a band
/// ("under" / "on" / "over" / "no_data") together with the actual value, the
/// target range and an optional delta.
class AdherenceRow {
  final double? actual;
  final double? targetMin;
  final double? targetMax;
  final double? deltaPct;
  final String status;

  AdherenceRow({
    required this.status,
    this.actual,
    this.targetMin,
    this.targetMax,
    this.deltaPct,
  });

  factory AdherenceRow.fromJson(Map<String, dynamic> json) {
    final target =
        (json['target'] as Map?)?.cast<String, dynamic>() ?? const {};
    return AdherenceRow(
      status: (json['status'] as String?) ?? 'no_data',
      actual: _asDouble(json['actual']),
      targetMin: _asDouble(target['min']),
      targetMax: _asDouble(target['max']),
      deltaPct: _asDouble(json['delta_pct']),
    );
  }
}

class Adherence {
  final Map<String, AdherenceRow> rows;

  Adherence(this.rows);

  factory Adherence.fromJson(Map<String, dynamic> json) {
    final out = <String, AdherenceRow>{};
    for (final entry in json.entries) {
      final v = entry.value;
      if (v is Map<String, dynamic>) {
        out[entry.key] = AdherenceRow.fromJson(v);
      }
    }
    return Adherence(out);
  }
}

class DailySummary {
  final String date;
  final String tz;
  final Totals totals;
  final List<MealEntry> entries;
  final Adherence? adherence;
  final String? goalSource;
  final String? phaseName;

  DailySummary({
    required this.date,
    required this.tz,
    required this.totals,
    required this.entries,
    this.adherence,
    this.goalSource,
    this.phaseName,
  });

  factory DailySummary.fromJson(Map<String, dynamic> json) => DailySummary(
        date: json['date'] as String,
        tz: json['tz'] as String,
        totals: Totals.fromJson(
            (json['totals'] as Map).cast<String, dynamic>()),
        entries: ((json['entries'] as List?) ?? const [])
            .map((e) => MealEntry.fromJson(
                (e as Map).cast<String, dynamic>()))
            .toList(),
        adherence: json['adherence'] is Map
            ? Adherence.fromJson(
                (json['adherence'] as Map).cast<String, dynamic>())
            : null,
        goalSource: json['goal_source'] as String?,
        phaseName: json['phase_name'] as String?,
      );
}

class GoalRange {
  final double? min;
  final double? max;

  GoalRange({this.min, this.max});

  factory GoalRange.fromJson(Map<String, dynamic> json) => GoalRange(
        min: _asDouble(json['min']),
        max: _asDouble(json['max']),
      );
}

/// Read-only view of the user's nutrition goals. The app does not let the
/// user edit these in v1 — the agent does.
class Goals {
  final Map<String, GoalRange> ranges;

  Goals(this.ranges);

  factory Goals.fromJson(Map<String, dynamic> json) {
    final out = <String, GoalRange>{};
    for (final entry in json.entries) {
      final v = entry.value;
      if (v is Map<String, dynamic>) {
        out[entry.key] = GoalRange.fromJson(v);
      }
    }
    return Goals(out);
  }
}
