import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../domain/models.dart';
import '../../state/app_providers.dart';
import '../../state/recent_provider.dart';
import '../../state/today_provider.dart';

/// Today's meals and hydration entries in one scrollable list, newest first.
/// Tapping a meal opens an edit/delete sheet; tapping a hydration entry offers
/// delete only.
class RecentPage extends ConsumerWidget {
  const RecentPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final recent = ref.watch(recentProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('Recent')),
      body: RefreshIndicator(
        onRefresh: () async {
          ref.invalidate(recentProvider);
          await ref.read(recentProvider.future);
        },
        child: recent.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => ListView(children: [
            const SizedBox(height: 120),
            Center(child: Text('$e')),
          ]),
          data: (items) => items.isEmpty
              ? ListView(children: const [
                  SizedBox(height: 120),
                  Center(child: Text('Nothing logged today.')),
                ])
              : ListView.separated(
                  itemCount: items.length,
                  separatorBuilder: (_, _) => const Divider(height: 1),
                  itemBuilder: (context, i) => _tile(context, ref, items[i]),
                ),
        ),
      ),
    );
  }

  Widget _tile(BuildContext context, WidgetRef ref, RecentItem item) {
    final time = DateFormat.Hm().format(item.loggedAt.toLocal());
    switch (item) {
      case RecentMeal(:final meal):
        return ListTile(
          leading: const Icon(Icons.restaurant_outlined),
          title: Text(meal.effectiveName),
          subtitle: Text('${meal.quantityG.toStringAsFixed(0)} g'
              '${meal.mealType != null ? ' · ${meal.mealType}' : ''} · $time'),
          onTap: () => _editMeal(context, ref, meal),
        );
      case RecentHydration(:final entry):
        return ListTile(
          leading: const Icon(Icons.local_drink_outlined),
          title: Text('${entry.quantityMl.toStringAsFixed(0)} ml water'),
          subtitle: Text(time),
          onTap: () => _deleteHydration(context, ref, entry),
        );
    }
  }

  Future<void> _editMeal(
      BuildContext context, WidgetRef ref, MealEntry meal) async {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (_) => _MealEditSheet(meal: meal),
    );
    ref.invalidate(recentProvider);
    ref.invalidate(todayProvider);
  }

  Future<void> _deleteHydration(
      BuildContext context, WidgetRef ref, HydrationEntry entry) async {
    final ok = await showModalBottomSheet<bool>(
      context: context,
      builder: (sheetContext) => Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('${entry.quantityMl.toStringAsFixed(0)} ml'),
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity,
              child: FilledButton.icon(
                style: FilledButton.styleFrom(
                    backgroundColor: Theme.of(context).colorScheme.error),
                icon: const Icon(Icons.delete_outline),
                label: const Text('Delete'),
                onPressed: () => Navigator.of(sheetContext).pop(true),
              ),
            ),
          ],
        ),
      ),
    );
    if (ok == true) {
      await ref.read(repositoryProvider).enqueueDeleteHydration(entry.id);
      ref.invalidate(recentProvider);
      ref.invalidate(hydrationDailyProvider);
    }
  }
}

const _mealTypes = ['breakfast', 'lunch', 'dinner', 'snack'];

class _MealEditSheet extends ConsumerStatefulWidget {
  final MealEntry meal;
  const _MealEditSheet({required this.meal});

  @override
  ConsumerState<_MealEditSheet> createState() => _MealEditSheetState();
}

class _MealEditSheetState extends ConsumerState<_MealEditSheet> {
  late final TextEditingController _qty =
      TextEditingController(text: widget.meal.quantityG.toStringAsFixed(0));
  late String _mealType = widget.meal.mealType ?? 'snack';

  @override
  void dispose() {
    _qty.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
          16, 0, 16, 16 + MediaQuery.of(context).viewInsets.bottom),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(widget.meal.effectiveName,
              style: Theme.of(context).textTheme.titleLarge),
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
                    border: OutlineInputBorder(),
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: DropdownButtonFormField<String>(
                  initialValue:
                      _mealTypes.contains(_mealType) ? _mealType : null,
                  decoration: const InputDecoration(
                    labelText: 'Meal',
                    border: OutlineInputBorder(),
                  ),
                  items: [
                    for (final t in _mealTypes)
                      DropdownMenuItem(value: t, child: Text(t)),
                  ],
                  onChanged: (v) => setState(() => _mealType = v ?? _mealType),
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              TextButton.icon(
                icon: Icon(Icons.delete_outline,
                    color: Theme.of(context).colorScheme.error),
                label: Text('Delete',
                    style:
                        TextStyle(color: Theme.of(context).colorScheme.error)),
                onPressed: () async {
                  await ref
                      .read(repositoryProvider)
                      .enqueueDeleteMeal(widget.meal.id);
                  if (context.mounted) Navigator.of(context).pop();
                },
              ),
              const Spacer(),
              FilledButton(
                onPressed: () async {
                  await ref.read(repositoryProvider).enqueuePatchMeal(
                        widget.meal.id,
                        quantityG: double.tryParse(_qty.text),
                        mealType: _mealType,
                      );
                  if (context.mounted) Navigator.of(context).pop();
                },
                child: const Text('Save'),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
