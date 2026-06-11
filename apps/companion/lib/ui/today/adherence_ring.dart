import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../domain/models.dart';

/// Maps an adherence band to a colour. Green = on, amber = under, red = over,
/// grey = no data. (Per the goals-rendering spec.)
Color bandColor(BuildContext context, String status) {
  switch (status) {
    case 'on':
      return Colors.green.shade600;
    case 'under':
      return Colors.amber.shade700;
    case 'over':
      return Colors.red.shade600;
    default:
      return Theme.of(context).disabledColor;
  }
}

/// A single nutrient ring: a coloured arc whose sweep reflects actual/target,
/// with the rounded actual value in the centre and the nutrient label below.
class AdherenceRing extends StatelessWidget {
  final String label;
  final String unit;
  final AdherenceRow row;

  const AdherenceRing({
    super.key,
    required this.label,
    required this.unit,
    required this.row,
  });

  double get _fraction {
    final actual = row.actual;
    if (actual == null) return 0;
    final ceiling = row.targetMax ?? row.targetMin;
    if (ceiling == null || ceiling == 0) return 0;
    return (actual / ceiling).clamp(0.0, 1.0);
  }

  @override
  Widget build(BuildContext context) {
    final color = bandColor(context, row.status);
    final actual = row.actual;
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(
          width: 64,
          height: 64,
          child: CustomPaint(
            painter: _RingPainter(
              fraction: _fraction,
              color: color,
              trackColor: Theme.of(context).colorScheme.surfaceContainerHighest,
            ),
            child: Center(
              child: Text(
                actual == null ? '—' : _short(actual),
                style: Theme.of(context).textTheme.labelLarge,
              ),
            ),
          ),
        ),
        const SizedBox(height: 4),
        Text(label, style: Theme.of(context).textTheme.labelSmall),
      ],
    );
  }

  static String _short(double v) {
    if (v >= 1000) return '${(v / 1000).toStringAsFixed(1)}k';
    if (v == v.roundToDouble()) return v.toStringAsFixed(0);
    return v.toStringAsFixed(1);
  }
}

class _RingPainter extends CustomPainter {
  final double fraction;
  final Color color;
  final Color trackColor;

  _RingPainter({
    required this.fraction,
    required this.color,
    required this.trackColor,
  });

  @override
  void paint(Canvas canvas, Size size) {
    const stroke = 6.0;
    final rect = Offset.zero & size;
    final center = rect.center;
    final radius = (size.shortestSide - stroke) / 2;

    final track = Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = stroke
      ..color = trackColor;
    canvas.drawCircle(center, radius, track);

    final arc = Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = stroke
      ..strokeCap = StrokeCap.round
      ..color = color;
    canvas.drawArc(
      Rect.fromCircle(center: center, radius: radius),
      -math.pi / 2,
      2 * math.pi * fraction,
      false,
      arc,
    );
  }

  @override
  bool shouldRepaint(_RingPainter old) =>
      old.fraction != fraction || old.color != color;
}

/// A small coloured dot for a tracked micronutrient that has adherence data
/// but isn't promoted to a full ring.
class AdherenceDot extends StatelessWidget {
  final String label;
  final AdherenceRow row;

  const AdherenceDot({super.key, required this.label, required this.row});

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          width: 8,
          height: 8,
          decoration: BoxDecoration(
            color: bandColor(context, row.status),
            shape: BoxShape.circle,
          ),
        ),
        const SizedBox(width: 4),
        Text(label, style: Theme.of(context).textTheme.labelSmall),
      ],
    );
  }
}
