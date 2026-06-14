import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/net/idempotency.dart';
import '../domain/chat.dart';
import 'app_providers.dart';

/// Inserts or coalesces a tool event into [tools] (mutated in place): a call's
/// `started` event inserts a chip and its matching `ok`/`error` event (same
/// `id`) replaces it — so one chip transitions running→done with no stale
/// "running" leftover. An empty `id` falls back to append.
void upsertToolEvent(List<ChatToolEvent> tools, ChatToolEvent ev) {
  final i = ev.id.isEmpty ? -1 : tools.indexWhere((t) => t.id == ev.id);
  if (i >= 0) {
    tools[i] = ev;
  } else {
    tools.add(ev);
  }
}

/// One active conversation. The streaming bubble's text lives in
/// [streamingText] while a turn is in flight; tool chips for the current turn
/// live in [tools]. [error] holds the last turn's failure code (retryable).
class ChatState {
  final List<ChatMessage> messages;
  final String? streamingText;
  final List<ChatToolEvent> tools;
  final String? error;

  /// A pending write-confirm proposal awaiting the user's decision. Non-null
  /// while the session is paused; the composer stays usable (typing implicitly
  /// rejects it server-side).
  final ChatPending? pending;

  const ChatState({
    this.messages = const [],
    this.streamingText,
    this.tools = const [],
    this.error,
    this.pending,
  });

  bool get streaming => streamingText != null;

  ChatState copyWith({
    List<ChatMessage>? messages,
    String? streamingText,
    bool clearStreaming = false,
    List<ChatToolEvent>? tools,
    String? error,
    bool clearError = false,
    ChatPending? pending,
    bool clearPending = false,
  }) {
    return ChatState(
      messages: messages ?? this.messages,
      streamingText: clearStreaming ? null : (streamingText ?? this.streamingText),
      tools: tools ?? this.tools,
      error: clearError ? null : (error ?? this.error),
      pending: clearPending ? null : (pending ?? this.pending),
    );
  }
}

class ChatNotifier extends Notifier<ChatState> {
  // _conversationId keys the local Drift cache (scrollback); _sessionId is the
  // server-side conversation the backend persists turns into. The server is the
  // source of truth — Drift is only a render cache.
  late String _conversationId;
  String? _sessionId;
  String? _lastUserText;

  @override
  ChatState build() {
    _conversationId = newIdempotencyKey();
    return const ChatState();
  }

  /// The active server session id, or null before the first turn. Lets the
  /// session list reset the screen when the open conversation is deleted.
  String? get activeSessionId => _sessionId;

  /// Starts a fresh conversation. Old messages stay in Drift for scrollback;
  /// the next send opens a new server session.
  void newChat() {
    _conversationId = newIdempotencyKey();
    _sessionId = null;
    _lastUserText = null;
    state = const ChatState();
  }

  /// The pending proposal awaiting confirmation, or null. Exposed so the card
  /// can read it.
  ChatPending? get pending => state.pending;

  /// Reopens a past session: loads its transcript, adopts its server
  /// `session_id`, and makes it the active conversation so new turns append to
  /// it. Returns false (leaving the current screen untouched) on a fetch
  /// failure. No-op while a turn is streaming.
  Future<bool> openSession(ChatSessionSummary session) async {
    if (state.streaming) return false;
    final ChatSessionDetail detail;
    try {
      detail = await ref.read(chatClientProvider).getSession(session.id);
    } catch (_) {
      return false;
    }
    _conversationId = newIdempotencyKey();
    _sessionId = session.id;
    _lastUserText = null;
    // Rebuild the proposal card from the persisted pending state (cold-open).
    state = ChatState(messages: detail.messages, pending: detail.pending);
    return true;
  }

  /// Sends [text] as a user turn and streams the assistant reply.
  Future<void> send(String text) async {
    final trimmed = text.trim();
    if (trimmed.isEmpty || state.streaming) return;
    final user = ChatMessage(
      id: newIdempotencyKey(),
      role: ChatRole.user,
      content: trimmed,
      createdAt: DateTime.now(),
    );
    _lastUserText = trimmed;
    // Sending while a proposal is pending implicitly rejects it (the server
    // appends declined tool_results and proceeds), so drop the card optimistically.
    state = state.copyWith(
      messages: [...state.messages, user],
      streamingText: '',
      tools: const [],
      clearError: true,
      clearPending: true,
    );
    await _persist(user);
    await _runTurn(trimmed);
  }

  /// Resolves a pending proposal: applies the per-call [approvals] (tool_id →
  /// approve) via the confirm endpoint and resumes streaming the continuation.
  /// No-op when nothing is pending or a turn is already streaming.
  Future<void> confirm(Map<String, bool> approvals) async {
    if (state.pending == null || _sessionId == null || state.streaming) return;
    state = state.copyWith(
      streamingText: '',
      tools: const [],
      clearError: true,
      clearPending: true,
    );
    await _runStream(
      ref.read(chatClientProvider).confirm(sessionId: _sessionId!, decisions: approvals),
    );
  }

  /// Re-runs the last user turn after a failure. The backend resumes from the
  /// persisted session and replays any completed tool writes idempotently.
  Future<void> retry() async {
    if (_lastUserText == null || state.streaming) return;
    state = state.copyWith(streamingText: '', tools: const [], clearError: true);
    await _runTurn(_lastUserText!);
  }

  /// Ensures a server session exists, creating one lazily on the first turn.
  /// Returns false (and surfaces a retryable error) when creation fails.
  Future<bool> _ensureSession() async {
    if (_sessionId != null) return true;
    final id = await ref.read(chatClientProvider).createSession();
    if (id == null) {
      state = state.copyWith(clearStreaming: true, error: 'chat_session_error');
      return false;
    }
    _sessionId = id;
    return true;
  }

  Future<void> _runTurn(String message) async {
    if (!await _ensureSession()) return;
    await _runStream(ref.read(chatClientProvider).stream(sessionId: _sessionId!, message: message));
  }

  /// Consumes a chat/confirm SSE event stream, driving the streaming bubble,
  /// tool chips, and — when the loop pauses — the proposal card. Shared by the
  /// send and confirm paths.
  Future<void> _runStream(Stream<ChatEvent> events) async {
    final buffer = StringBuffer();
    final tools = <ChatToolEvent>[];
    ChatPending? pending;
    try {
      await for (final ev in events) {
        switch (ev) {
          case ChatTextEvent(:final text):
            buffer.write(text);
            state = state.copyWith(streamingText: buffer.toString());
          case ChatToolEvent():
            upsertToolEvent(tools, ev);
            state = state.copyWith(tools: List.of(tools));
          case ChatProposalEvent():
            pending = ev.pending;
          case ChatDoneEvent(:final message, :final awaitingConfirmation):
            final content = message.isNotEmpty ? message : buffer.toString();
            if (awaitingConfirmation) {
              await _finalizePaused(content, pending);
            } else {
              await _finalize(content);
            }
            return;
          case ChatErrorEvent(:final code):
            state = state.copyWith(clearStreaming: true, error: code);
            return;
        }
      }
      // Stream ended without a done/error event — treat as a dropped turn.
      if (state.streaming) {
        state = state.copyWith(clearStreaming: true, error: 'stream_dropped');
      }
    } catch (e) {
      state = state.copyWith(clearStreaming: true, error: 'stream_dropped');
    }
  }

  Future<void> _finalize(String content) async {
    final assistant = ChatMessage(
      id: newIdempotencyKey(),
      role: ChatRole.assistant,
      content: content,
      createdAt: DateTime.now(),
    );
    state = state.copyWith(
      messages: [...state.messages, assistant],
      clearStreaming: true,
      tools: const [],
      clearPending: true,
    );
    await _persist(assistant);
    await ref.read(appDatabaseProvider).chatMessagesDao.pruneConversations();
  }

  /// Finalizes a turn that paused awaiting confirmation: the assistant's text
  /// (its narration of what it's about to do) becomes a bubble, the proposal
  /// card is surfaced, and streaming stops so the composer re-enables. The
  /// pending writes have NOT fired.
  Future<void> _finalizePaused(String content, ChatPending? pending) async {
    final messages = [...state.messages];
    if (content.isNotEmpty) {
      final assistant = ChatMessage(
        id: newIdempotencyKey(),
        role: ChatRole.assistant,
        content: content,
        createdAt: DateTime.now(),
      );
      messages.add(assistant);
      await _persist(assistant);
    }
    state = state.copyWith(
      messages: messages,
      clearStreaming: true,
      tools: const [],
      pending: pending,
    );
  }

  Future<void> _persist(ChatMessage m) {
    return ref.read(appDatabaseProvider).chatMessagesDao.insertMessage(
          id: m.id,
          conversationId: _conversationId,
          role: chatRoleName(m.role),
          content: m.content,
          createdAt: m.createdAt,
        );
  }
}

final chatProvider =
    NotifierProvider<ChatNotifier, ChatState>(ChatNotifier.new);
