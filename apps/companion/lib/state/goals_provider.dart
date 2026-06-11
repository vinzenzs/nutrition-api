import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../domain/models.dart';
import 'app_providers.dart';

/// Read-only goals. Null means "no goals set" (`GET /goals` → `{"goals":null}`),
/// which the Today screen renders as raw totals + a "Set goals via the
/// assistant" hint. The app never writes goals — that is the agent's job.
final goalsProvider = FutureProvider<Goals?>((ref) {
  return ref.watch(repositoryProvider).fetchGoals();
});
