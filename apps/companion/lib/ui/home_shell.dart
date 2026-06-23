import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/app_providers.dart';
import '../state/garmin_provider.dart';
import '../state/push_provider.dart';
import '../state/recent_provider.dart';
import '../state/today_provider.dart';
import '../state/train_provider.dart';
import '../data/db/app_database.dart';
import '../data/push/push_messaging.dart';
import '../data/sync/replay_triggers.dart';
import 'camera/camera_page.dart';
import 'chat/chat_page.dart';
import 'garmin/garmin_connect_sheet.dart';
import 'recent/recent_page.dart';
import 'today/today_page.dart';
import 'train/train_page.dart';

/// The five-screen shell: Today, Train, Camera, Recent, Chat. Train is the
/// fueling-lens-on-training surface (add-companion-train-screen); Chat is the
/// in-app coach (add-companion-chat).
class HomeShell extends ConsumerStatefulWidget {
  const HomeShell({super.key});

  @override
  ConsumerState<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends ConsumerState<HomeShell> {
  int _index = 0;
  ReplayTriggers? _triggers;
  final List<StreamSubscription<PushMessage>> _pushSubs = [];

  static const _pages = [
    TodayPage(),
    TrainPage(),
    CameraPage(),
    RecentPage(),
    ChatPage(),
  ];

  @override
  void initState() {
    super.initState();
    // Start the outbox replay triggers (foreground, connectivity, backstop)
    // now that we're past pairing and the DB/api are wired.
    final worker = ref.read(outboxWorkerProvider);
    _triggers = ReplayTriggers(worker);
    _triggers!.start();
    _syncWidgetConfig();
    _startPush();
  }

  /// Now that we're paired, register the FCM token and route relogin pushes
  /// into the Garmin connect sheet. Registration is fire-and-forget (best-effort,
  /// re-tried on next launch); message subscriptions live for the shell's life.
  void _startPush() {
    ref.read(pushProvider.notifier).register();
    final messaging = ref.read(pushMessagingProvider);
    _pushSubs.add(messaging.onMessageOpenedApp.listen(_onPushOpened));
    _pushSubs.add(messaging.onMessage.listen(_onPushForeground));
    // Cold start via a tray tap.
    messaging.getInitialMessage().then((m) {
      if (m != null) _onPushOpened(m);
    });
  }

  /// A tray tap (background or cold start): open the Garmin sheet directly.
  void _onPushOpened(PushMessage m) {
    if (intentFor(m) == PushIntent.garminRelogin) _openGarmin();
  }

  /// Foreground arrival: Android suppresses the tray entry, so surface a
  /// lightweight prompt rather than silently dropping it.
  void _onPushForeground(PushMessage m) {
    if (intentFor(m) != PushIntent.garminRelogin || !mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: const Text('Garmin needs you to reconnect.'),
        action: SnackBarAction(label: 'Reconnect', onPressed: _openGarmin),
      ),
    );
  }

  void _openGarmin() {
    if (!mounted) return;
    ref.invalidate(garminSyncProvider); // refresh status when we surface the flow
    showGarminSheet(context);
  }

  /// Mirror glass size, hydration goal, and the Drift DB path into the native
  /// widget so its tap worker can run and spill over offline taps correctly.
  Future<void> _syncWidgetConfig() async {
    final prefs = ref.read(prefsProvider);
    final dbPath = await AppDatabase.resolveDbPath();
    await ref.read(widgetBridgeProvider).setConfig(
          glassSizeMl: prefs.glassSizeMl,
          hydrationGoalMl: prefs.hydrationGoalMl,
          driftDbPath: dbPath,
        );
  }

  @override
  void dispose() {
    _triggers?.stop();
    for (final sub in _pushSubs) {
      sub.cancel();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: IndexedStack(index: _index, children: _pages),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: (i) {
          setState(() => _index = i);
          // Reconcile the destination so writes made on another tab (which go
          // through the async outbox) show up. By the time the user taps over,
          // the outbox has flushed; refresh() avoids a loading-spinner flash.
          switch (i) {
            case 0:
              ref.read(todayProvider.notifier).refresh();
              ref.read(hydrationDailyProvider.notifier).refresh();
            case 1:
              ref.read(trainProvider.notifier).refresh();
            case 3:
              ref.read(recentProvider.notifier).refresh();
          }
        },
        destinations: const [
          NavigationDestination(
              icon: Icon(Icons.today_outlined),
              selectedIcon: Icon(Icons.today),
              label: 'Today'),
          NavigationDestination(
              icon: Icon(Icons.fitness_center_outlined),
              selectedIcon: Icon(Icons.fitness_center),
              label: 'Train'),
          NavigationDestination(
              icon: Icon(Icons.photo_camera_outlined),
              selectedIcon: Icon(Icons.photo_camera),
              label: 'Camera'),
          NavigationDestination(
              icon: Icon(Icons.list_alt_outlined),
              selectedIcon: Icon(Icons.list_alt),
              label: 'Recent'),
          NavigationDestination(
              icon: Icon(Icons.chat_bubble_outline),
              selectedIcon: Icon(Icons.chat_bubble),
              label: 'Chat'),
        ],
      ),
    );
  }
}
