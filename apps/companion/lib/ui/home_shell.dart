import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/app_providers.dart';
import '../state/recent_provider.dart';
import '../state/today_provider.dart';
import '../state/train_provider.dart';
import '../data/db/app_database.dart';
import '../data/sync/replay_triggers.dart';
import 'camera/camera_page.dart';
import 'chat/chat_page.dart';
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
