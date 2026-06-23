import 'package:firebase_messaging/firebase_messaging.dart';

/// A push reduced to what the app acts on: its `data` payload. The display
/// `notification` (title/body) is rendered by the system tray on Android, so
/// the app only inspects `data` (the backend's `action` routing key).
class PushMessage {
  final Map<String, dynamic> data;
  const PushMessage(this.data);

  /// The backend routing key, e.g. `garmin_relogin`. Null/absent for messages
  /// the app doesn't act on.
  String? get action => data['action'] as String?;

  factory PushMessage.fromRemote(RemoteMessage m) => PushMessage(m.data);
}

/// Outcome of the OS notification-permission gate. The app registers its token
/// regardless, so [denied] is a UI hint, not a blocker.
enum PushPermission { granted, denied }

/// Small port over `firebase_messaging` so the push provider is unit-testable
/// with a fake — matching how the app fakes `Repository`/`ApiClient` elsewhere.
abstract class PushMessaging {
  /// Requests the runtime notification permission (Android 13+).
  Future<PushPermission> requestPermission();

  /// The current FCM registration token, or null if unavailable.
  Future<String?> getToken();

  /// Fires when FCM rotates the registration token.
  Stream<String> get onTokenRefresh;

  /// Messages delivered while the app is foregrounded.
  Stream<PushMessage> get onMessage;

  /// A tap on the tray notification that brought the app to the foreground.
  Stream<PushMessage> get onMessageOpenedApp;

  /// The message that cold-started the app via a tray tap, if any.
  Future<PushMessage?> getInitialMessage();
}

/// The real implementation over `FirebaseMessaging.instance`. Firebase is
/// touched lazily (only inside method bodies) so constructing the adapter never
/// requires `Firebase.initializeApp()` to have run — keeps tests/headless safe.
class FirebaseMessagingAdapter implements PushMessaging {
  FirebaseMessaging get _fm => FirebaseMessaging.instance;

  @override
  Future<PushPermission> requestPermission() async {
    final settings = await _fm.requestPermission();
    final status = settings.authorizationStatus;
    final granted = status == AuthorizationStatus.authorized ||
        status == AuthorizationStatus.provisional;
    return granted ? PushPermission.granted : PushPermission.denied;
  }

  @override
  Future<String?> getToken() => _fm.getToken();

  @override
  Stream<String> get onTokenRefresh => _fm.onTokenRefresh;

  @override
  Stream<PushMessage> get onMessage =>
      FirebaseMessaging.onMessage.map(PushMessage.fromRemote);

  @override
  Stream<PushMessage> get onMessageOpenedApp =>
      FirebaseMessaging.onMessageOpenedApp.map(PushMessage.fromRemote);

  @override
  Future<PushMessage?> getInitialMessage() async {
    final m = await _fm.getInitialMessage();
    return m == null ? null : PushMessage.fromRemote(m);
  }
}
