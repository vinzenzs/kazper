// Garmin connection + sync-status domain models
// (add-garmin-connect-and-sync-status). Read-only mirrors of the backend
// `GET /garmin/sync-status` shape; the login flow carries no model (it's a
// trigger + MFA code, with the bridge holding the credentials).

/// One sync run as reported by the bridge.
class GarminSyncRun {
  final String id;
  final String status; // running | success | error
  final DateTime startedAt;
  final DateTime? finishedAt;
  final String? windowFrom;
  final String? windowTo;
  final String? error;

  const GarminSyncRun({
    required this.id,
    required this.status,
    required this.startedAt,
    this.finishedAt,
    this.windowFrom,
    this.windowTo,
    this.error,
  });

  bool get isRunning => status == 'running';
  bool get isError => status == 'error';

  factory GarminSyncRun.fromJson(Map<String, dynamic> j) => GarminSyncRun(
        id: j['id'] as String,
        status: j['status'] as String,
        startedAt: DateTime.parse(j['started_at'] as String),
        finishedAt: j['finished_at'] == null
            ? null
            : DateTime.parse(j['finished_at'] as String),
        windowFrom: j['window_from'] as String?,
        windowTo: j['window_to'] as String?,
        error: j['error'] as String?,
      );
}

/// The `GET /garmin/sync-status` response.
class GarminSyncStatus {
  final GarminSyncRun? latest;
  final DateTime? lastSuccessfulAt;
  final bool isStale;

  const GarminSyncStatus({
    this.latest,
    this.lastSuccessfulAt,
    required this.isStale,
  });

  factory GarminSyncStatus.fromJson(Map<String, dynamic> j) => GarminSyncStatus(
        latest: j['latest'] == null
            ? null
            : GarminSyncRun.fromJson((j['latest'] as Map).cast<String, dynamic>()),
        lastSuccessfulAt: j['last_successful_at'] == null
            ? null
            : DateTime.parse(j['last_successful_at'] as String),
        isStale: (j['is_stale'] as bool?) ?? true,
      );
}
