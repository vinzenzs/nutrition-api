import 'dart:math';

/// Mint a UUID-v4. Used as the `Idempotency-Key` header for outbox rows.
///
/// Keep it dependency-free: the package is small enough to skip the `uuid`
/// crate. Math.Random.secure() is good enough for opaque-id purposes — these
/// keys don't need to be unguessable, just collision-resistant within a TTL.
String newIdempotencyKey() {
  final rng = Random.secure();
  final bytes = List<int>.generate(16, (_) => rng.nextInt(256));
  // RFC 4122 v4: set version (4) and variant (10xx) bits.
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;

  String hex(int b) => b.toRadixString(16).padLeft(2, '0');
  final h = bytes.map(hex).toList();
  return '${h[0]}${h[1]}${h[2]}${h[3]}-'
      '${h[4]}${h[5]}-'
      '${h[6]}${h[7]}-'
      '${h[8]}${h[9]}-'
      '${h[10]}${h[11]}${h[12]}${h[13]}${h[14]}${h[15]}';
}
