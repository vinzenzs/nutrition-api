import 'package:dio/dio.dart';

import '../auth/token_store.dart';

/// Thin wrapper around Dio that pulls baseUrl + bearer token from
/// [TokenStore] on every request. Idempotency-Key is added by the caller
/// (the outbox row carries it); the client adds a default User-Agent.
class ApiClient {
  final Dio dio;
  final TokenStore tokenStore;

  ApiClient({required this.tokenStore, Dio? dio})
      : dio = dio ?? Dio() {
    this.dio.options
      ..connectTimeout = const Duration(seconds: 10)
      ..receiveTimeout = const Duration(seconds: 15)
      ..headers['User-Agent'] = 'nutrition-companion/0.1';

    this.dio.interceptors.add(InterceptorsWrapper(
      onRequest: (options, handler) async {
        final baseUrl = await tokenStore.getBaseUrl();
        final token = await tokenStore.getToken();
        if (baseUrl != null && options.baseUrl.isEmpty) {
          options.baseUrl = baseUrl;
        }
        if (token != null) {
          options.headers['Authorization'] = 'Bearer $token';
        }
        handler.next(options);
      },
    ));
  }

  /// Sends an outbox row's request. Returns the raw response so the worker
  /// can classify by status code.
  Future<Response<dynamic>> send({
    required String method,
    required String path,
    required List<int> body,
    required String idempotencyKey,
  }) {
    return dio.request(
      path,
      data: body,
      options: Options(
        method: method,
        contentType: 'application/json',
        responseType: ResponseType.json,
        headers: {'Idempotency-Key': idempotencyKey},
        validateStatus: (_) => true, // worker classifies; never throw on status
      ),
    );
  }
}
