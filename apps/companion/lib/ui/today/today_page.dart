import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/models.dart';
import '../../state/app_providers.dart';
import '../../state/today_provider.dart';
import '../nutrient_labels.dart';
import '../settings/settings_sheet.dart';
import 'adherence_ring.dart';

/// Glance surface: adherence rings (or raw totals when goals are unset), a
/// hydration progress card, and the last three meals. The floating action
/// logs one configured glass of water without leaving the screen.
class TodayPage extends ConsumerWidget {
  const TodayPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final today = ref.watch(todayProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Today'),
        actions: [
          IconButton(
            icon: const Icon(Icons.settings_outlined),
            tooltip: 'Settings',
            onPressed: () => showSettingsSheet(context),
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton.extended(
        icon: const Icon(Icons.local_drink_outlined),
        label: const Text('Water'),
        onPressed: () => _logGlass(context, ref),
      ),
      body: RefreshIndicator(
        onRefresh: () => ref.read(todayProvider.notifier).refresh(),
        child: today.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => _ErrorBody(message: '$e'),
          data: (summary) => summary == null
              ? const _ErrorBody(message: 'No data yet.')
              : _TodayBody(summary: summary),
        ),
      ),
    );
  }

  Future<void> _logGlass(BuildContext context, WidgetRef ref) async {
    final prefs = ref.read(prefsProvider);
    final ml = prefs.glassSizeMl;
    await ref
        .read(repositoryProvider)
        .enqueueHydration(quantityMl: ml.toDouble());
    if (!context.mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('Logged ${ml}ml')),
    );
    ref.invalidate(hydrationDailyProvider);

    // Push the fresh total into the widget's snapshot (best-effort).
    final hydration = await ref.read(hydrationDailyProvider.future);
    await ref.read(widgetBridgeProvider).updateSnapshot(
          date: todayDate(),
          totalMl: hydration.totalMl,
          goalMl: prefs.hydrationGoalMl.toDouble(),
        );
  }
}

class _TodayBody extends ConsumerWidget {
  final DailySummary summary;
  const _TodayBody({required this.summary});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final adherence = summary.adherence;
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 96),
      children: [
        if (adherence != null && adherence.rows.isNotEmpty)
          _RingsBlock(adherence: adherence)
        else
          _RawTotalsBlock(summary: summary),
        const SizedBox(height: 16),
        const _HydrationCard(),
        const SizedBox(height: 24),
        Text('Recent meals', style: Theme.of(context).textTheme.titleMedium),
        const SizedBox(height: 8),
        ..._recentMeals(context),
      ],
    );
  }

  List<Widget> _recentMeals(BuildContext context) {
    final meals = summary.entries.reversed.take(3).toList();
    if (meals.isEmpty) {
      return [
        Padding(
          padding: const EdgeInsets.symmetric(vertical: 8),
          child: Text('Nothing logged yet.',
              style: Theme.of(context).textTheme.bodyMedium),
        ),
      ];
    }
    return meals
        .map((m) => ListTile(
              contentPadding: EdgeInsets.zero,
              dense: true,
              leading: const Icon(Icons.restaurant_outlined),
              title: Text(m.effectiveName),
              subtitle: Text('${m.quantityG.toStringAsFixed(0)} g'
                  '${m.mealType != null ? ' · ${m.mealType}' : ''}'),
            ))
        .toList();
  }
}

class _RingsBlock extends StatelessWidget {
  final Adherence adherence;
  const _RingsBlock({required this.adherence});

  @override
  Widget build(BuildContext context) {
    final rings = <Widget>[];
    for (final key in ringNutrients) {
      final row = adherence.rows[key];
      if (row == null) continue;
      rings.add(AdherenceRing(label: labelFor(key), unit: unitFor(key), row: row));
    }

    // Any other tracked micronutrient with adherence data becomes a dot.
    final dots = <Widget>[];
    for (final entry in adherence.rows.entries) {
      if (ringNutrients.contains(entry.key)) continue;
      if (entry.value.status == 'no_data') continue;
      dots.add(AdherenceDot(label: labelFor(entry.key), row: entry.value));
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 16,
          runSpacing: 16,
          alignment: WrapAlignment.spaceBetween,
          children: rings,
        ),
        if (dots.isNotEmpty) ...[
          const SizedBox(height: 16),
          Wrap(spacing: 12, runSpacing: 8, children: dots),
        ],
      ],
    );
  }
}

class _RawTotalsBlock extends StatelessWidget {
  final DailySummary summary;
  const _RawTotalsBlock({required this.summary});

  @override
  Widget build(BuildContext context) {
    final t = summary.totals;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _row(context, 'Energy', '${t.kcal.toStringAsFixed(0)} kcal'),
            _row(context, 'Protein', '${t.proteinG.toStringAsFixed(0)} g'),
            _row(context, 'Carbs', '${t.carbsG.toStringAsFixed(0)} g'),
            _row(context, 'Fat', '${t.fatG.toStringAsFixed(0)} g'),
            const SizedBox(height: 8),
            Row(
              children: [
                Icon(Icons.info_outline,
                    size: 16, color: Theme.of(context).hintColor),
                const SizedBox(width: 6),
                Expanded(
                  child: Text('Set goals via the assistant',
                      style: Theme.of(context).textTheme.bodySmall),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _row(BuildContext context, String label, String value) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [Text(label), Text(value)],
        ),
      );
}

class _HydrationCard extends ConsumerWidget {
  const _HydrationCard();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final hydration = ref.watch(hydrationDailyProvider);
    final goalMl = ref.watch(prefsProvider).hydrationGoalMl;

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: hydration.when(
          loading: () => const LinearProgressIndicator(),
          error: (_, _) => const Text('Hydration unavailable'),
          data: (h) {
            final fraction = goalMl == 0 ? 0.0 : (h.totalMl / goalMl).clamp(0.0, 1.0);
            return Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    const Text('Hydration'),
                    Text('${h.totalMl.toStringAsFixed(0)} / $goalMl ml'),
                  ],
                ),
                const SizedBox(height: 8),
                ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: LinearProgressIndicator(value: fraction, minHeight: 8),
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

class _ErrorBody extends StatelessWidget {
  final String message;
  const _ErrorBody({required this.message});

  @override
  Widget build(BuildContext context) {
    return ListView(
      children: [
        const SizedBox(height: 120),
        Center(child: Text(message)),
      ],
    );
  }
}
