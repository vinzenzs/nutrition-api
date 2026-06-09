import 'package:flutter/services.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:nutrition_companion/data/auth/token_store.dart';

class _MockSecureStorage extends Mock implements FlutterSecureStorage {}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() {
    // mocktail needs fallback Any matchers for named args used with verify().
    registerFallbackValue('');
  });

  group('SecureTokenStore', () {
    late _MockSecureStorage storage;
    late MethodChannel channel;
    late List<MethodCall> bridgeCalls;
    late SecureTokenStore store;

    setUp(() {
      storage = _MockSecureStorage();
      channel = const MethodChannel(
          'com.corelyr.nutrition_companion/token_bridge.test');
      bridgeCalls = [];

      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, (call) async {
        bridgeCalls.add(call);
        return null;
      });

      store = SecureTokenStore(storage: storage, channel: channel);

      when(() => storage.write(
              key: any(named: 'key'), value: any(named: 'value')))
          .thenAnswer((_) async {});
      when(() => storage.delete(key: any(named: 'key')))
          .thenAnswer((_) async {});
    });

    tearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    test('pair writes both keys and mirrors to the bridge', () async {
      await store.pair(
          baseUrl: 'http://192.168.1.10:8080', token: 'tok-123');

      verify(() => storage.write(
              key: 'base_url', value: 'http://192.168.1.10:8080'))
          .called(1);
      verify(() => storage.write(key: 'mobile_api_token', value: 'tok-123'))
          .called(1);

      expect(bridgeCalls, hasLength(1));
      expect(bridgeCalls.single.method, 'mirror');
      expect(bridgeCalls.single.arguments, {
        'base_url': 'http://192.168.1.10:8080',
        'token': 'tok-123',
      });
    });

    test('getToken / getBaseUrl read through the storage layer', () async {
      when(() => storage.read(key: 'mobile_api_token'))
          .thenAnswer((_) async => 'tok-xyz');
      when(() => storage.read(key: 'base_url'))
          .thenAnswer((_) async => 'http://example:8080');

      expect(await store.getToken(), 'tok-xyz');
      expect(await store.getBaseUrl(), 'http://example:8080');
    });

    test('clear deletes both keys and notifies the bridge', () async {
      await store.clear();

      verify(() => storage.delete(key: 'mobile_api_token')).called(1);
      verify(() => storage.delete(key: 'base_url')).called(1);

      expect(bridgeCalls, hasLength(1));
      expect(bridgeCalls.single.method, 'clear');
    });

    test('round-trip: pair then getToken returns the stored value',
        () async {
      String? stored;
      when(() => storage.write(
              key: 'mobile_api_token', value: any(named: 'value')))
          .thenAnswer((invocation) async {
        stored = invocation.namedArguments[#value] as String?;
      });
      when(() => storage.read(key: 'mobile_api_token'))
          .thenAnswer((_) async => stored);

      await store.pair(baseUrl: 'http://example', token: 'tok-round-trip');
      expect(await store.getToken(), 'tok-round-trip');
    });

    test('platform-channel failure during pair is swallowed', () async {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, (call) async {
        throw PlatformException(code: 'NO_BRIDGE');
      });

      // Should not throw — bridge errors are tolerated.
      await store.pair(baseUrl: 'http://example', token: 'tok-no-bridge');

      // The secure-storage writes still happened.
      verify(() => storage.write(
              key: 'base_url', value: 'http://example'))
          .called(1);
      verify(() => storage.write(
              key: 'mobile_api_token', value: 'tok-no-bridge'))
          .called(1);
    });
  });
}
