import 'dart:convert';

import 'package:drift/drift.dart';

import '../app_database.dart';

part 'training_day_dao.g.dart';

/// Cache accessor for the Train screen's assembled [TrainingDay] payload,
/// keyed by local date. Stores the whole assembled envelope as JSON (the same
/// pattern as RecentSummaryDao) so the screen can paint stale-while-revalidate.
@DriftAccessor(tables: [TrainingDayCache])
class TrainingDayDao extends DatabaseAccessor<AppDatabase>
    with _$TrainingDayDaoMixin {
  TrainingDayDao(super.db);

  Future<void> upsertForDate({
    required String date,
    required Map<String, dynamic> payload,
  }) {
    return into(trainingDayCache).insertOnConflictUpdate(
      TrainingDayCacheCompanion.insert(
        date: date,
        payloadJson: jsonEncode(payload),
        refreshedAt: DateTime.now(),
      ),
    );
  }

  Future<Map<String, dynamic>?> getForDate(String date) async {
    final row = await (select(trainingDayCache)
          ..where((r) => r.date.equals(date)))
        .getSingleOrNull();
    if (row == null) return null;
    return jsonDecode(row.payloadJson) as Map<String, dynamic>;
  }
}
