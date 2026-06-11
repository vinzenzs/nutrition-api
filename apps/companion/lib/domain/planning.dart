// Models for the planning surfaces (planned meals + shopping list) the chat
// produces. Hand-rolled JSON, mirroring the backend /plan and /shopping shapes.

double? _asDouble(dynamic v) => v == null ? null : (v as num).toDouble();

/// One planned meal from `GET /plan`. `status` is planned | eaten | skipped.
class PlannedMeal {
  final String id;
  final String planDate;
  final String slot;
  final String? productId;
  final String? productName;
  final double? quantityG;
  final String status;
  final String? mealEntryId;

  PlannedMeal({
    required this.id,
    required this.planDate,
    required this.slot,
    required this.status,
    this.productId,
    this.productName,
    this.quantityG,
    this.mealEntryId,
  });

  factory PlannedMeal.fromJson(Map<String, dynamic> json) => PlannedMeal(
        id: json['id'] as String,
        planDate: json['plan_date'] as String,
        slot: json['slot'] as String,
        status: json['status'] as String,
        productId: json['product_id'] as String?,
        productName: json['product_name'] as String?,
        quantityG: _asDouble(json['quantity_g']),
        mealEntryId: json['meal_entry_id'] as String?,
      );

  PlannedMeal copyWith({String? status}) => PlannedMeal(
        id: id,
        planDate: planDate,
        slot: slot,
        status: status ?? this.status,
        productId: productId,
        productName: productName,
        quantityG: quantityG,
        mealEntryId: mealEntryId,
      );
}

/// One shopping-list item from `GET /shopping/items`.
class ShoppingItem {
  final String id;
  final String name;
  final String? quantityText;
  final bool checked;

  ShoppingItem({
    required this.id,
    required this.name,
    required this.checked,
    this.quantityText,
  });

  factory ShoppingItem.fromJson(Map<String, dynamic> json) => ShoppingItem(
        id: json['id'] as String,
        name: json['name'] as String,
        checked: (json['checked'] as bool?) ?? false,
        quantityText: json['quantity_text'] as String?,
      );

  ShoppingItem copyWith({bool? checked}) => ShoppingItem(
        id: id,
        name: name,
        checked: checked ?? this.checked,
        quantityText: quantityText,
      );
}
