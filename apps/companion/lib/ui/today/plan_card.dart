import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/planning.dart';
import '../../state/plan_provider.dart';

/// Today's planned meals on the Today screen. One-tap "Ate it" logs a real meal
/// (through the outbox); skip marks it skipped. Absent entirely when there is
/// no plan for today. Actions disable while a prior eaten/skip is still syncing.
class PlanCard extends ConsumerWidget {
  const PlanCard({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final plan = ref.watch(planProvider);
    if (plan.entries.isEmpty) return const SizedBox.shrink();

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.only(left: 4, bottom: 4),
              child: Text("Today's plan",
                  style: Theme.of(context).textTheme.titleMedium),
            ),
            for (final e in plan.entries)
              _PlanRow(entry: e, pending: plan.pending.contains(e.id)),
          ],
        ),
      ),
    );
  }
}

class _PlanRow extends ConsumerWidget {
  final PlannedMeal entry;
  final bool pending;
  const _PlanRow({required this.entry, required this.pending});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final notifier = ref.read(planProvider.notifier);
    final planned = entry.status == 'planned';
    final subtitle = [
      entry.slot,
      if (entry.quantityG != null) '${entry.quantityG!.toStringAsFixed(0)} g',
    ].join(' · ');

    Widget trailing;
    if (pending) {
      trailing = const Text('syncing…',
          style: TextStyle(fontStyle: FontStyle.italic, color: Colors.grey));
    } else if (planned) {
      trailing = Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          TextButton(
            onPressed: () => notifier.skip(entry.id),
            child: const Text('Skip'),
          ),
          FilledButton(
            onPressed: () => notifier.markEaten(entry.id),
            child: const Text('Ate it'),
          ),
        ],
      );
    } else {
      trailing = Chip(
        visualDensity: VisualDensity.compact,
        label: Text(entry.status),
        avatar: Icon(
          entry.status == 'eaten' ? Icons.check : Icons.block,
          size: 16,
        ),
      );
    }

    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 4),
      dense: true,
      title: Text(entry.productName ?? 'Planned meal'),
      subtitle: Text(subtitle),
      trailing: trailing,
    );
  }
}
