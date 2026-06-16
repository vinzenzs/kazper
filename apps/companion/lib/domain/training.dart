/// Domain models for the Train screen (add-companion-train-screen, Band 1).
///
/// A [TrainingDay] is the app-assembled view of one day's prescribed
/// session(s) and the fuel each demands — composed in the repository from
/// `GET /context/training` (the session headers), `GET /workouts/{id}/program`
/// (resolved targets), and `GET /race-prep/recommend-workout-fuel` (the fuel).
/// The shape here is the app's own; it round-trips through the Drift cache as
/// JSON, so fromJson/toJson are symmetric over THIS schema, not the backend's.
library;

class TrainingDay {
  final String date; // YYYY-MM-DD (local)
  final List<TrainSession> sessions;

  const TrainingDay({required this.date, required this.sessions});

  factory TrainingDay.fromJson(Map<String, dynamic> j) => TrainingDay(
        date: j['date'] as String,
        sessions: ((j['sessions'] as List?) ?? const [])
            .map((e) => TrainSession.fromJson((e as Map).cast<String, dynamic>()))
            .toList(),
      );

  Map<String, dynamic> toJson() => {
        'date': date,
        'sessions': sessions.map((s) => s.toJson()).toList(),
      };
}

class TrainSession {
  final String workoutId;
  final String sport;
  final String? name;
  final DateTime startedAt;
  final double durationMin;

  /// Compact resolved-target lines for display, e.g. `["Z4 · 230–268 W"]`.
  /// For a multisport session these are per-segment, e.g. `["bike · 230–268 W"]`.
  final List<String> targets;

  /// The fueling the session demands; null when the recommendation failed.
  final SessionFuel? fuel;

  const TrainSession({
    required this.workoutId,
    required this.sport,
    required this.name,
    required this.startedAt,
    required this.durationMin,
    required this.targets,
    required this.fuel,
  });

  factory TrainSession.fromJson(Map<String, dynamic> j) => TrainSession(
        workoutId: j['workout_id'] as String,
        sport: j['sport'] as String,
        name: j['name'] as String?,
        startedAt: DateTime.parse(j['started_at'] as String),
        durationMin: (j['duration_min'] as num).toDouble(),
        targets: ((j['targets'] as List?) ?? const []).cast<String>(),
        fuel: j['fuel'] == null
            ? null
            : SessionFuel.fromJson((j['fuel'] as Map).cast<String, dynamic>()),
      );

  Map<String, dynamic> toJson() => {
        'workout_id': workoutId,
        'sport': sport,
        'name': name,
        'started_at': startedAt.toIso8601String(),
        'duration_min': durationMin,
        'targets': targets,
        'fuel': fuel?.toJson(),
      };
}

/// Display-ready pre/intra/post fueling lines plus any disclosure notes
/// (e.g. the Z2-default note when a planned session has no TSS).
class SessionFuel {
  final List<String> pre;
  final List<String> intra;
  final List<String> post;
  final List<String> notes;

  const SessionFuel({
    required this.pre,
    required this.intra,
    required this.post,
    required this.notes,
  });

  factory SessionFuel.fromJson(Map<String, dynamic> j) => SessionFuel(
        pre: ((j['pre'] as List?) ?? const []).cast<String>(),
        intra: ((j['intra'] as List?) ?? const []).cast<String>(),
        post: ((j['post'] as List?) ?? const []).cast<String>(),
        notes: ((j['notes'] as List?) ?? const []).cast<String>(),
      );

  Map<String, dynamic> toJson() => {
        'pre': pre,
        'intra': intra,
        'post': post,
        'notes': notes,
      };
}
