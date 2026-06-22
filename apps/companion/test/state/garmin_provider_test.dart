import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:kazper/data/net/api_client.dart';
import 'package:kazper/state/app_providers.dart';
import 'package:kazper/state/garmin_provider.dart';

class _MockApiClient extends Mock implements ApiClient {}

class _MockDio extends Mock implements Dio {}

Response<Map<String, dynamic>> _resp(int status, Map<String, dynamic> data) =>
    Response(
      requestOptions: RequestOptions(path: '/'),
      statusCode: status,
      data: data,
    );

ProviderContainer _container(Dio dio) {
  final api = _MockApiClient();
  when(() => api.dio).thenReturn(dio);
  final c = ProviderContainer(overrides: [apiClientProvider.overrideWithValue(api)]);
  addTearDown(c.dispose);
  return c;
}

void main() {
  void stubLogin(Dio dio, Response<Map<String, dynamic>> resp) {
    when(() => dio.post<Map<String, dynamic>>('/garmin/login',
        options: any(named: 'options'))).thenAnswer((_) async => resp);
  }

  void stubMfa(Dio dio, Response<Map<String, dynamic>> resp) {
    when(() => dio.post<Map<String, dynamic>>('/garmin/login/mfa',
            data: any(named: 'data'), options: any(named: 'options')))
        .thenAnswer((_) async => resp);
  }

  test('connect → needs_mfa moves to awaitingMfa', () async {
    final dio = _MockDio();
    stubLogin(dio, _resp(200, {'needs_mfa': true}));
    final c = _container(dio);

    await c.read(garminConnectProvider.notifier).connect();
    expect(c.read(garminConnectProvider).phase, GarminConnectPhase.awaitingMfa);
  });

  test('connect → logged_in connects directly', () async {
    final dio = _MockDio();
    stubLogin(dio, _resp(200, {'logged_in': true}));
    final c = _container(dio);

    await c.read(garminConnectProvider.notifier).connect();
    expect(c.read(garminConnectProvider).phase, GarminConnectPhase.connected);
  });

  test('connect → 503 shows disabled', () async {
    final dio = _MockDio();
    stubLogin(dio, _resp(503, {'error': 'garmin_disabled'}));
    final c = _container(dio);

    await c.read(garminConnectProvider.notifier).connect();
    expect(c.read(garminConnectProvider).phase, GarminConnectPhase.disabled);
  });

  test('submitMfa wrong code stays on entry with an error', () async {
    final dio = _MockDio();
    stubMfa(dio, _resp(400, {'error': 'mfa_invalid'}));
    final c = _container(dio);

    await c.read(garminConnectProvider.notifier).submitMfa('000000');
    final s = c.read(garminConnectProvider);
    expect(s.phase, GarminConnectPhase.awaitingMfa);
    expect(s.error, isNotNull);
  });

  test('submitMfa success connects', () async {
    final dio = _MockDio();
    stubMfa(dio, _resp(200, {'logged_in': true}));
    final c = _container(dio);

    await c.read(garminConnectProvider.notifier).submitMfa('123456');
    expect(c.read(garminConnectProvider).phase, GarminConnectPhase.connected);
  });

  test('sync status: 503 yields null (not configured)', () async {
    final dio = _MockDio();
    when(() => dio.get<Map<String, dynamic>>('/garmin/sync-status',
            options: any(named: 'options')))
        .thenAnswer((_) async => _resp(503, {'error': 'garmin_disabled'}));
    final c = _container(dio);

    final status = await c.read(garminSyncProvider.future);
    expect(status, isNull);
  });

  test('sync status: parses latest + last_successful_at', () async {
    final dio = _MockDio();
    when(() => dio.get<Map<String, dynamic>>('/garmin/sync-status',
            options: any(named: 'options')))
        .thenAnswer((_) async => _resp(200, {
              'latest': {
                'id': 'r1',
                'status': 'error',
                'started_at': '2026-06-22T05:00:00Z',
                'finished_at': '2026-06-22T05:01:00Z',
                'error': 'boom',
              },
              'last_successful_at': '2026-06-21T05:00:00Z',
              'is_stale': true,
            }));
    final c = _container(dio);

    final status = await c.read(garminSyncProvider.future);
    expect(status, isNotNull);
    expect(status!.latest!.isError, isTrue);
    expect(status.lastSuccessfulAt, isNotNull);
    expect(status.isStale, isTrue);
  });
}
