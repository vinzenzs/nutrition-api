import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../state/shopping_provider.dart';

/// The shopping list — unchecked first, tap to check off, add an item, and
/// clear bought items. All writes go through the offline outbox (optimistic).
class ShoppingPage extends ConsumerWidget {
  const ShoppingPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final shopping = ref.watch(shoppingProvider);
    final hasChecked = shopping.items.any((i) => i.checked);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Shopping'),
        actions: [
          if (hasChecked)
            IconButton(
              tooltip: 'Clear bought',
              icon: const Icon(Icons.cleaning_services_outlined),
              onPressed: () => _confirmClear(context, ref),
            ),
        ],
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: () => _addItem(context, ref),
        child: const Icon(Icons.add),
      ),
      body: RefreshIndicator(
        onRefresh: () => ref.read(shoppingProvider.notifier).refresh(),
        child: shopping.loading && shopping.items.isEmpty
            ? const Center(child: CircularProgressIndicator())
            : shopping.items.isEmpty
                ? ListView(children: const [
                    SizedBox(height: 120),
                    Center(child: Text('Nothing on the list.')),
                  ])
                : ListView.builder(
                    itemCount: shopping.items.length,
                    itemBuilder: (context, i) {
                      final item = shopping.items[i];
                      return CheckboxListTile(
                        value: item.checked,
                        onChanged: (v) => ref
                            .read(shoppingProvider.notifier)
                            .setChecked(item.id, v ?? false),
                        title: Text(
                          item.name,
                          style: item.checked
                              ? const TextStyle(
                                  decoration: TextDecoration.lineThrough,
                                  color: Colors.grey)
                              : null,
                        ),
                        subtitle: item.quantityText != null
                            ? Text(item.quantityText!)
                            : null,
                        controlAffinity: ListTileControlAffinity.leading,
                        dense: true,
                      );
                    },
                  ),
      ),
    );
  }

  Future<void> _addItem(BuildContext context, WidgetRef ref) async {
    final controller = TextEditingController();
    final name = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Add item'),
        content: TextField(
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(hintText: 'e.g. Zwiebeln'),
          onSubmitted: (v) => Navigator.pop(ctx, v.trim()),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
          FilledButton(
              onPressed: () => Navigator.pop(ctx, controller.text.trim()),
              child: const Text('Add')),
        ],
      ),
    );
    if (name != null && name.isNotEmpty) {
      await ref.read(shoppingProvider.notifier).addItem(name);
    }
  }

  Future<void> _confirmClear(BuildContext context, WidgetRef ref) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Clear bought items?'),
        content: const Text('Removes every checked item from the list.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
          FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Clear')),
        ],
      ),
    );
    if (ok == true) {
      await ref.read(shoppingProvider.notifier).clearChecked();
    }
  }
}
