import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/models.dart';
import '../../state/app_providers.dart';

/// Confidence thresholds for the photo flow (per the camera spec).
const double kAutoCommitConfidence = 0.75;
const double kConfirmConfidence = 0.6;

/// Routes a `POST /meals/from_photo` result to the right confirmation UX:
///   - ≥0.75  auto-committed; show a card with Undo.
///   - 0.6–0.75 amber confirm (keep / edit / discard).
///   - <0.6   the auto-created entry is discarded; an editable sheet collects
///            corrected values and re-logs via `POST /meals/freeform`.
Future<void> showPhotoConfirm(
  BuildContext context,
  WidgetRef ref,
  PhotoMealResult result,
) async {
  if (result.confidence >= kAutoCommitConfidence) {
    return _showAutoCommit(context, ref, result);
  }
  if (result.confidence >= kConfirmConfidence) {
    return _showAmberConfirm(context, ref, result);
  }
  // Low confidence: discard the auto-created meal, edit from scratch.
  await ref.read(repositoryProvider).enqueueDeleteMeal(result.meal.id);
  if (!context.mounted) return;
  return showFreeformSheet(context, ref, seed: result.meal);
}

Future<void> _showAutoCommit(
  BuildContext context,
  WidgetRef ref,
  PhotoMealResult result,
) {
  return showModalBottomSheet<void>(
    context: context,
    builder: (sheetContext) => _ConfirmBanner(
      color: Colors.green.shade600,
      title: 'Logged: ${result.meal.effectiveName}',
      subtitle: 'Confidence ${(result.confidence * 100).round()}%',
      actions: [
        TextButton.icon(
          icon: const Icon(Icons.undo),
          label: const Text('Undo'),
          onPressed: () async {
            await ref
                .read(repositoryProvider)
                .enqueueDeleteMeal(result.meal.id);
            if (sheetContext.mounted) Navigator.of(sheetContext).pop();
          },
        ),
        FilledButton(
          onPressed: () => Navigator.of(sheetContext).pop(),
          child: const Text('OK'),
        ),
      ],
    ),
  );
}

Future<void> _showAmberConfirm(
  BuildContext context,
  WidgetRef ref,
  PhotoMealResult result,
) {
  return showModalBottomSheet<void>(
    context: context,
    builder: (sheetContext) => _ConfirmBanner(
      color: Colors.amber.shade700,
      title: result.meal.effectiveName,
      subtitle:
          'Confidence ${(result.confidence * 100).round()}% — keep or edit?',
      actions: [
        TextButton(
          onPressed: () async {
            await ref
                .read(repositoryProvider)
                .enqueueDeleteMeal(result.meal.id);
            if (sheetContext.mounted) Navigator.of(sheetContext).pop();
          },
          child: const Text('Discard'),
        ),
        OutlinedButton(
          onPressed: () async {
            await ref
                .read(repositoryProvider)
                .enqueueDeleteMeal(result.meal.id);
            if (!sheetContext.mounted) return;
            Navigator.of(sheetContext).pop();
            await showFreeformSheet(context, ref, seed: result.meal);
          },
          child: const Text('Edit'),
        ),
        FilledButton(
          onPressed: () => Navigator.of(sheetContext).pop(),
          child: const Text('Keep'),
        ),
      ],
    ),
  );
}

/// Editable meal sheet. Used by the low/amber photo bands (seeded from the
/// inference) and by the freeform "describe it" escape hatch (empty seed).
/// Submit calls `POST /meals/freeform`.
Future<void> showFreeformSheet(
  BuildContext context,
  WidgetRef ref, {
  MealEntry? seed,
}) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (_) => _EditableMealSheet(seed: seed),
  );
}

class _ConfirmBanner extends StatelessWidget {
  final Color color;
  final String title;
  final String subtitle;
  final List<Widget> actions;

  const _ConfirmBanner({
    required this.color,
    required this.title,
    required this.subtitle,
    required this.actions,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(20),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(width: 12, height: 12,
                  decoration: BoxDecoration(color: color, shape: BoxShape.circle)),
              const SizedBox(width: 8),
              Expanded(
                child: Text(title,
                    style: Theme.of(context).textTheme.titleMedium),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(subtitle, style: Theme.of(context).textTheme.bodySmall),
          const SizedBox(height: 16),
          Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              for (final a in actions) Padding(
                padding: const EdgeInsets.only(left: 8),
                child: a,
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _EditableMealSheet extends ConsumerStatefulWidget {
  final MealEntry? seed;
  const _EditableMealSheet({this.seed});

  @override
  ConsumerState<_EditableMealSheet> createState() => _EditableMealSheetState();
}

class _EditableMealSheetState extends ConsumerState<_EditableMealSheet> {
  late final TextEditingController _name;
  late final TextEditingController _qty;
  late final TextEditingController _kcal;
  late final TextEditingController _protein;
  late final TextEditingController _carbs;
  late final TextEditingController _fat;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    final seed = widget.seed;
    final n = seed?.effectiveNutrimentsPer100g;
    _name = TextEditingController(text: seed?.effectiveName ?? '');
    _qty = TextEditingController(text: (seed?.quantityG ?? 100).toStringAsFixed(0));
    _kcal = TextEditingController(text: _fmt(n?.kcal));
    _protein = TextEditingController(text: _fmt(n?.proteinG));
    _carbs = TextEditingController(text: _fmt(n?.carbsG));
    _fat = TextEditingController(text: _fmt(n?.fatG));
  }

  static String _fmt(double? v) => v == null ? '' : v.toStringAsFixed(1);

  @override
  void dispose() {
    for (final c in [_name, _qty, _kcal, _protein, _carbs, _fat]) {
      c.dispose();
    }
    super.dispose();
  }

  Future<void> _submit() async {
    final name = _name.text.trim();
    if (name.isEmpty) return;
    setState(() => _saving = true);
    await ref.read(repositoryProvider).enqueueFreeformMeal(
          name: name,
          quantityG: double.tryParse(_qty.text) ?? 100,
          mealType: mealTypeForNow(),
          loggedAt: DateTime.now(),
          kcal: double.tryParse(_kcal.text),
          proteinG: double.tryParse(_protein.text),
          carbsG: double.tryParse(_carbs.text),
          fatG: double.tryParse(_fat.text),
        );
    if (!mounted) return;
    Navigator.of(context).pop();
    ScaffoldMessenger.of(context)
        .showSnackBar(const SnackBar(content: Text('Meal logged')));
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
          Text('Describe the meal',
              style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 12),
          TextField(
            controller: _name,
            decoration: const InputDecoration(
                labelText: 'Name', border: OutlineInputBorder()),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(child: _num(_qty, 'Quantity (g)')),
              const SizedBox(width: 12),
              Expanded(child: _num(_kcal, 'kcal / 100g')),
            ],
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(child: _num(_protein, 'Protein')),
              const SizedBox(width: 8),
              Expanded(child: _num(_carbs, 'Carbs')),
              const SizedBox(width: 8),
              Expanded(child: _num(_fat, 'Fat')),
            ],
          ),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity,
            child: FilledButton(
              onPressed: _saving ? null : _submit,
              child: Text(_saving ? 'Saving…' : 'Log meal'),
            ),
          ),
        ],
      ),
    );
  }

  Widget _num(TextEditingController c, String label) => TextField(
        controller: c,
        keyboardType: const TextInputType.numberWithOptions(decimal: true),
        decoration: InputDecoration(
          labelText: label,
          isDense: true,
          border: const OutlineInputBorder(),
        ),
      );
}
