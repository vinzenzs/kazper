// Chat domain types: transcript messages and the five streamed event types the
// backend `POST /chat` SSE emits (text | tool | proposal | done | error).

enum ChatRole { user, assistant }

String chatRoleName(ChatRole r) => r == ChatRole.user ? 'user' : 'assistant';

ChatRole chatRoleFrom(String s) =>
    s == 'user' ? ChatRole.user : ChatRole.assistant;

class ChatMessage {
  final String id;
  final ChatRole role;
  final String content;
  final DateTime createdAt;

  ChatMessage({
    required this.id,
    required this.role,
    required this.content,
    required this.createdAt,
  });
}

/// A streamed event from the chat SSE response.
sealed class ChatEvent {}

/// An assistant text delta.
class ChatTextEvent extends ChatEvent {
  final String text;
  ChatTextEvent(this.text);
}

/// A tool's lifecycle: status is started | ok | error; summary is a short
/// human string (never raw bodies).
class ChatToolEvent extends ChatEvent {
  /// The upstream tool_use id; a call's started and ok/error events share it,
  /// so the UI coalesces them into one chip.
  final String id;
  final String name;
  final String status;
  final String summary;
  ChatToolEvent({required this.id, required this.name, required this.status, required this.summary});
  bool get isError => status == 'error';
}

/// One pending write-confirm call the coach proposed: a server-composed,
/// honest preview (never the model's prose) plus its identity and tier.
class ChatPendingCall {
  final String toolId;
  final String name;
  final String tier;
  final String preview;
  ChatPendingCall({
    required this.toolId,
    required this.name,
    required this.tier,
    required this.preview,
  });

  factory ChatPendingCall.fromJson(Map<String, dynamic> j) => ChatPendingCall(
        toolId: j['tool_id'] as String? ?? '',
        name: j['name'] as String? ?? '',
        tier: j['tier'] as String? ?? '',
        preview: j['preview'] as String? ?? '',
      );
}

/// A paused turn awaiting the user's per-call decision. The same shape arrives
/// live (the `proposal` SSE event) and on cold-open (`pending_confirmation` in
/// the session detail), so one card renders both.
class ChatPending {
  final String turnId;
  final List<ChatPendingCall> calls;
  ChatPending({required this.turnId, required this.calls});

  factory ChatPending.fromJson(Map<String, dynamic> j) => ChatPending(
        turnId: j['turn_id'] as String? ?? '',
        calls: [
          for (final c in (j['calls'] as List? ?? const []))
            if (c is Map<String, dynamic>) ChatPendingCall.fromJson(c),
        ],
      );
}

/// The loop paused awaiting confirmation: it lists the pending write-confirm
/// calls. Followed by a done event with stop_reason "awaiting_confirmation".
class ChatProposalEvent extends ChatEvent {
  final ChatPending pending;
  ChatProposalEvent(this.pending);
}

/// Terminates a successful stream with the full final message.
class ChatDoneEvent extends ChatEvent {
  final String message;
  final String stopReason;
  ChatDoneEvent({required this.message, required this.stopReason});
  bool get awaitingConfirmation => stopReason == 'awaiting_confirmation';
}

/// Terminates the stream with a typed code (e.g. chat_unavailable).
class ChatErrorEvent extends ChatEvent {
  final String code;
  final String message;
  ChatErrorEvent({required this.code, required this.message});
}

/// One row in the session-history list — the `GET /chat/sessions` header
/// (no transcript). Title is null when the backend hasn't named it yet.
class ChatSessionSummary {
  final String id;
  final String? title;
  final DateTime lastMessageAt;
  final DateTime createdAt;

  /// True when this session's trailing turn is paused awaiting a write
  /// confirmation — the history list badges it so the user can return and act.
  final bool awaitingConfirmation;

  ChatSessionSummary({
    required this.id,
    this.title,
    required this.lastMessageAt,
    required this.createdAt,
    this.awaitingConfirmation = false,
  });

  factory ChatSessionSummary.fromJson(Map<String, dynamic> j) =>
      ChatSessionSummary(
        id: j['id'] as String,
        title: j['title'] as String?,
        lastMessageAt:
            DateTime.parse(j['last_message_at'] as String).toLocal(),
        createdAt: DateTime.parse(j['created_at'] as String).toLocal(),
        awaitingConfirmation: j['awaiting_confirmation'] as bool? ?? false,
      );
}

/// A reopened session: its header, the reconstructed visible transcript, and —
/// when the session is paused — the pending proposal so the card is rebuilt on
/// cold-open (D9).
class ChatSessionDetail {
  final ChatSessionSummary summary;
  final List<ChatMessage> messages;
  final ChatPending? pending;
  ChatSessionDetail({required this.summary, required this.messages, this.pending});
}

/// A typed failure from the `/chat/sessions` read/manage calls.
class ChatSessionException implements Exception {
  final String code;
  const ChatSessionException(this.code);
  @override
  String toString() => 'ChatSessionException($code)';
}

/// Reconstructs visible chat bubbles from a session's stored turns. Each turn is
/// `{role, content}` where `content` is the verbatim Anthropic value: a JSON
/// string (plain user text) or a content-block array. User string turns become
/// user bubbles; assistant turns become a bubble from their `text` blocks;
/// `tool_use`-only assistant turns and `tool_result` user turns are dropped.
/// Pure and deterministic — timestamps are synthetic (unused by the transcript).
List<ChatMessage> reconstructTranscript(List<dynamic> turns) {
  final out = <ChatMessage>[];
  final epoch = DateTime.fromMillisecondsSinceEpoch(0);
  var i = 0;
  for (final raw in turns) {
    if (raw is Map) {
      final role = raw['role'];
      final content = raw['content'];
      if (role == 'user' && content is String) {
        out.add(ChatMessage(
            id: 'hist-$i', role: ChatRole.user, content: content, createdAt: epoch));
      } else if (role == 'assistant') {
        final text = _assistantText(content);
        if (text.isNotEmpty) {
          out.add(ChatMessage(
              id: 'hist-$i',
              role: ChatRole.assistant,
              content: text,
              createdAt: epoch));
        }
      }
      // user array content (tool_result) and assistant tool_use-only turns: skip.
    }
    i++;
  }
  return out;
}

/// Concatenates the `text` blocks of an assistant content value.
String _assistantText(dynamic content) {
  if (content is String) return content; // defensive — shouldn't happen
  if (content is! List) return '';
  final b = StringBuffer();
  for (final block in content) {
    if (block is Map && block['type'] == 'text' && block['text'] is String) {
      b.write(block['text'] as String);
    }
  }
  return b.toString();
}
