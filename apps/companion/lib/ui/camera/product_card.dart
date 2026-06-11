import 'package:flutter/material.dart';

import '../../domain/models.dart';

const _mealTypes = ['breakfast', 'lunch', 'dinner', 'snack'];

/// The scan-mode confirmation card. Pre-filled quantity (last logged → serving
/// → 100) and time-of-day meal type; "Log" is the only required tap.
class ProductCard extends StatefulWidget {
  final Product product;
  final double quantityG;
  final String mealType;
  final ValueChanged<double> onQuantityChanged;
  final ValueChanged<String> onMealTypeChanged;
  final VoidCallback onLog;
  final VoidCallback onDismiss;

  const ProductCard({
    super.key,
    required this.product,
    required this.quantityG,
    required this.mealType,
    required this.onQuantityChanged,
    required this.onMealTypeChanged,
    required this.onLog,
    required this.onDismiss,
  });

  @override
  State<ProductCard> createState() => _ProductCardState();
}

class _ProductCardState extends State<ProductCard> {
  late final TextEditingController _qty =
      TextEditingController(text: widget.quantityG.toStringAsFixed(0));

  @override
  void dispose() {
    _qty.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final p = widget.product;
    final kcalPer100 = p.nutrimentsPer100g.kcal;
    return Card(
      margin: const EdgeInsets.all(16),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(p.name,
                          style: Theme.of(context).textTheme.titleMedium),
                      if (p.brand != null)
                        Text(p.brand!,
                            style: Theme.of(context).textTheme.bodySmall),
                    ],
                  ),
                ),
                IconButton(
                    onPressed: widget.onDismiss, icon: const Icon(Icons.close)),
              ],
            ),
            const SizedBox(height: 4),
            Text('${kcalPer100.toStringAsFixed(0)} kcal / 100 g',
                style: Theme.of(context).textTheme.bodySmall),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _qty,
                    keyboardType:
                        const TextInputType.numberWithOptions(decimal: true),
                    decoration: const InputDecoration(
                      labelText: 'Quantity',
                      suffixText: 'g',
                      isDense: true,
                      border: OutlineInputBorder(),
                    ),
                    onChanged: (v) {
                      final q = double.tryParse(v);
                      if (q != null) widget.onQuantityChanged(q);
                    },
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: DropdownButtonFormField<String>(
                    initialValue: widget.mealType,
                    isDense: true,
                    decoration: const InputDecoration(
                      labelText: 'Meal',
                      border: OutlineInputBorder(),
                    ),
                    items: [
                      for (final t in _mealTypes)
                        DropdownMenuItem(value: t, child: Text(t)),
                    ],
                    onChanged: (v) {
                      if (v != null) widget.onMealTypeChanged(v);
                    },
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            SizedBox(
              width: double.infinity,
              child: FilledButton.icon(
                icon: const Icon(Icons.check),
                label: const Text('Log'),
                onPressed: widget.onLog,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
