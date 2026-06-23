import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:kazper/data/net/api_client.dart';
import 'package:kazper/data/push/push_messaging.dart';
import 'package:kazper/state/app_providers.dart';
import 'package:kazper/state/push_provider.dart';

class _MockApiClient extends Mock implements ApiClient {}

class _MockDio extends Mock implements Dio {}

/// Behaviour-only fake messaging port — no Firebase. Tests set [permission] and
/// [token], then drive [emitRefresh] to simulate a rotated registration token.
class FakePushMessaging implements PushMessaging {
  PushPermission permission = PushPermission.granted;
  String? token = 'tok-1';
  final _refresh = StreamController<String>.broadcast();

  @override
  Future<PushPermission> requestPermission() async => permission;
  @override
  Future<String?> getToken() async => token;
  @override
  Stream<String> get onTokenRefresh => _refresh.stream;
  @override
  Stream<PushMessage> get onMessage => const Stream.empty();
  @override
  Stream<PushMessage> get onMessageOpenedApp => const Stream.empty();
  @override
  Future<PushMessage?> getInitialMessage() async => null;

  void emitRefresh(String t) => _refresh.add(t);
  void dispose() => _refresh.close();
}

Response<dynamic> _resp(int status) =>
    Response(requestOptions: RequestOptions(path: '/'), statusCode: status);

void main() {
  ProviderContainer container(Dio dio, FakePushMessaging messaging) {
    final api = _MockApiClient();
    when(() => api.dio).thenReturn(dio);
    final c = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(api),
      pushMessagingProvider.overrideWithValue(messaging),
    ]);
    addTearDown(c.dispose);
    addTearDown(messaging.dispose);
    return c;
  }

  void stubPost(Dio dio) {
    when(() => dio.post<dynamic>('/push/tokens',
            data: any(named: 'data'), options: any(named: 'options')))
        .thenAnswer((_) async => _resp(201));
  }

  void stubDelete(Dio dio) {
    when(() => dio.delete<dynamic>('/push/tokens',
            data: any(named: 'data'), options: any(named: 'options')))
        .thenAnswer((_) async => _resp(204));
  }

  test('registers the token on pair', () async {
    final dio = _MockDio();
    stubPost(dio);
    final messaging = FakePushMessaging();
    final c = container(dio, messaging);

    await c.read(pushProvider.notifier).register();

    final captured = verify(() => dio.post<dynamic>('/push/tokens',
        data: captureAny(named: 'data'),
        options: any(named: 'options'))).captured;
    expect(captured.single, {'token': 'tok-1', 'platform': 'android'});
    expect(c.read(pushProvider).notificationsEnabled, isTrue);
  });

  test('re-registers on token refresh', () async {
    final dio = _MockDio();
    stubPost(dio);
    final messaging = FakePushMessaging();
    final c = container(dio, messaging);

    await c.read(pushProvider.notifier).register();
    messaging.emitRefresh('tok-2');
    await pumpEventQueue();

    final tokens = verify(() => dio.post<dynamic>('/push/tokens',
            data: captureAny(named: 'data'), options: any(named: 'options')))
        .captured
        .map((d) => (d as Map)['token'])
        .toList();
    expect(tokens, ['tok-1', 'tok-2']);
  });

  test('deregisters the last token on unpair', () async {
    final dio = _MockDio();
    stubPost(dio);
    stubDelete(dio);
    final messaging = FakePushMessaging();
    final c = container(dio, messaging);

    await c.read(pushProvider.notifier).register();
    await c.read(pushProvider.notifier).deregister();

    final captured = verify(() => dio.delete<dynamic>('/push/tokens',
        data: captureAny(named: 'data'),
        options: any(named: 'options'))).captured;
    expect(captured.single, {'token': 'tok-1'});
  });

  test('registers even when the permission is denied', () async {
    final dio = _MockDio();
    stubPost(dio);
    final messaging = FakePushMessaging()..permission = PushPermission.denied;
    final c = container(dio, messaging);

    await c.read(pushProvider.notifier).register();

    verify(() => dio.post<dynamic>('/push/tokens',
        data: any(named: 'data'), options: any(named: 'options'))).called(1);
    expect(c.read(pushProvider).notificationsEnabled, isFalse);
  });

  group('intentFor', () {
    test('garmin_relogin maps to the Garmin relogin intent', () {
      expect(intentFor(const PushMessage({'action': 'garmin_relogin'})),
          PushIntent.garminRelogin);
    });

    test('an unknown action is a no-op intent', () {
      expect(intentFor(const PushMessage({'action': 'something_else'})),
          PushIntent.none);
    });

    test('an absent action is a no-op intent', () {
      expect(intentFor(const PushMessage({})), PushIntent.none);
    });
  });
}
