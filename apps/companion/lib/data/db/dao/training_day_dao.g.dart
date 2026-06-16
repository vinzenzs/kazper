// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'training_day_dao.dart';

// ignore_for_file: type=lint
mixin _$TrainingDayDaoMixin on DatabaseAccessor<AppDatabase> {
  $TrainingDayCacheTable get trainingDayCache =>
      attachedDatabase.trainingDayCache;
  TrainingDayDaoManager get managers => TrainingDayDaoManager(this);
}

class TrainingDayDaoManager {
  final _$TrainingDayDaoMixin _db;
  TrainingDayDaoManager(this._db);
  $$TrainingDayCacheTableTableManager get trainingDayCache =>
      $$TrainingDayCacheTableTableManager(
        _db.attachedDatabase,
        _db.trainingDayCache,
      );
}
