import 'package:flutter/services.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

abstract class TokenStore {
  Future<String?> getToken();
  Future<String?> getBaseUrl();
  Future<void> pair({required String baseUrl, required String token});
  Future<void> clear();
}

class SecureTokenStore implements TokenStore {
  static const _tokenKey = 'mobile_api_token';
  static const _baseUrlKey = 'base_url';
  static const _bridgeChannel =
      MethodChannel('com.corelyr.nutrition_companion/token_bridge');

  final FlutterSecureStorage _storage;
  final MethodChannel _channel;

  SecureTokenStore({
    FlutterSecureStorage? storage,
    MethodChannel? channel,
  })  : _storage = storage ?? const FlutterSecureStorage(),
        _channel = channel ?? _bridgeChannel;

  @override
  Future<String?> getToken() => _storage.read(key: _tokenKey);

  @override
  Future<String?> getBaseUrl() => _storage.read(key: _baseUrlKey);

  @override
  Future<void> pair({required String baseUrl, required String token}) async {
    await _storage.write(key: _baseUrlKey, value: baseUrl);
    await _storage.write(key: _tokenKey, value: token);
    // Mirror into EncryptedSharedPreferences so the Kotlin widget worker can
    // read the same token without going through Flutter. Tolerate failure —
    // the Flutter app still works without the widget.
    try {
      await _channel.invokeMethod<void>('mirror', {
        'base_url': baseUrl,
        'token': token,
      });
    } on PlatformException {
      // Channel not registered (e.g. unit tests) — fine, the in-app path
      // still uses flutter_secure_storage.
    } on MissingPluginException {
      // Same — bridge not wired in this build.
    }
  }

  @override
  Future<void> clear() async {
    await _storage.delete(key: _tokenKey);
    await _storage.delete(key: _baseUrlKey);
    try {
      await _channel.invokeMethod<void>('clear');
    } on PlatformException {
      // Ignore — see pair().
    } on MissingPluginException {
      // Ignore — see pair().
    }
  }
}
