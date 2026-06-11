import 'package:shared_preferences/shared_preferences.dart';

/// Non-secure local preferences. Holds the configurable glass size and the
/// hydration daily goal — both are UI tuning knobs, never the bearer token
/// (that lives in `flutter_secure_storage`; see the spec's secure-storage
/// requirement).
class Prefs {
  static const _glassSizeKey = 'glass_size_ml';
  static const _hydrationGoalKey = 'hydration_goal_ml';

  static const defaultGlassSizeMl = 250;
  static const defaultHydrationGoalMl = 2500;

  final SharedPreferences _prefs;

  Prefs(this._prefs);

  static Future<Prefs> open() async => Prefs(await SharedPreferences.getInstance());

  int get glassSizeMl => _prefs.getInt(_glassSizeKey) ?? defaultGlassSizeMl;

  Future<void> setGlassSizeMl(int ml) => _prefs.setInt(_glassSizeKey, ml);

  int get hydrationGoalMl =>
      _prefs.getInt(_hydrationGoalKey) ?? defaultHydrationGoalMl;

  Future<void> setHydrationGoalMl(int ml) =>
      _prefs.setInt(_hydrationGoalKey, ml);
}
