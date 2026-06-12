import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../domain/chat.dart';
import 'app_providers.dart';
import 'chat_provider.dart';

/// The session-history list. Online-only — there is no offline cache; a failed
/// fetch surfaces [error] (retryable) rather than a stale list.
class SessionsState {
  final List<ChatSessionSummary> sessions;
  final bool loading;
  final String? error;

  const SessionsState({
    this.sessions = const [],
    this.loading = false,
    this.error,
  });

  SessionsState copyWith({
    List<ChatSessionSummary>? sessions,
    bool? loading,
    String? error,
    bool clearError = false,
  }) {
    return SessionsState(
      sessions: sessions ?? this.sessions,
      loading: loading ?? this.loading,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

class SessionsNotifier extends Notifier<SessionsState> {
  @override
  SessionsState build() {
    _load();
    return const SessionsState(loading: true);
  }

  // _load awaits before any state write so it is safe to call from build()
  // (mutating state during build is illegal).
  Future<void> _load() async {
    try {
      final list = await ref.read(chatClientProvider).listSessions();
      state = SessionsState(sessions: list, loading: false);
    } catch (_) {
      state = const SessionsState(loading: false, error: 'load_failed');
    }
  }

  Future<void> refresh() async {
    state = state.copyWith(loading: true, clearError: true);
    await _load();
  }

  /// Deletes a session, optimistically removing it. If it was the active
  /// conversation, the Chat screen resets to a fresh chat. Re-syncs on failure.
  Future<void> delete(String id) async {
    final wasActive = ref.read(chatProvider.notifier).activeSessionId == id;
    state = state.copyWith(
      sessions: state.sessions.where((s) => s.id != id).toList(),
    );
    try {
      await ref.read(chatClientProvider).deleteSession(id);
      if (wasActive) ref.read(chatProvider.notifier).newChat();
    } catch (_) {
      await _load();
    }
  }

  /// Renames a session, then refreshes so the new title (or auto-title) shows.
  Future<void> rename(String id, String title) async {
    try {
      await ref.read(chatClientProvider).renameSession(id, title);
    } finally {
      await _load();
    }
  }
}

final sessionsProvider =
    NotifierProvider<SessionsNotifier, SessionsState>(SessionsNotifier.new);
