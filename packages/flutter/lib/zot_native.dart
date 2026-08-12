import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:flutter/services.dart';

class ZotSessionOptions {
  const ZotSessionOptions({
    required this.provider,
    this.apiKey,
    this.accessToken,
    this.accountId,
    this.model,
    this.systemPrompt,
  });
  final String provider;
  final String? apiKey, accessToken, accountId, model, systemPrompt;
  Map<String, Object?> toMap() => {
        'provider': provider,
        'apiKey': apiKey,
        'accessToken': accessToken,
        'accountId': accountId,
        'model': model,
        'systemPrompt': systemPrompt,
      }..removeWhere((_, value) => value == null);
}

class ZotEvent {
  const ZotEvent(this.type, this.payload);
  final String type;
  final Map<String, dynamic> payload;
}

class ZotSession {
  ZotSession._(this.id);
  final String id;
  Stream<ZotEvent> get events =>
      ZotNative._events.where((value) => value['sessionId'] == id).map((value) {
        final event = Map<String, dynamic>.from(value['event'] as Map);
        return ZotEvent(event['type'] as String, event);
      });
  Future<void> prompt(String message) => ZotNative._methods.invokeMethod(
        'prompt',
        {'sessionId': id, 'message': message},
      );
  Future<void> abort() =>
      ZotNative._methods.invokeMethod('abort', {'sessionId': id});
  Future<void> close() =>
      ZotNative._methods.invokeMethod('closeSession', {'sessionId': id});
  Future<String> exportHistory() async =>
      (await ZotNative._methods
          .invokeMethod<String>('exportHistory', {'sessionId': id})) ??
      '[]';
  Future<void> importHistory(String history) => ZotNative._methods.invokeMethod(
        'importHistory',
        {'sessionId': id, 'history': history},
      );
}

class ZotDesktopClient {
  ZotDesktopClient._(this._process) {
    _stdoutSubscription = _process.stdout
        .transform(utf8.decoder)
        .transform(const LineSplitter())
        .listen(_receive,
            onError: _failAll,
            onDone: () => _failAll(StateError('zot bridge closed')));
    _stderrSubscription = _process.stderr.listen((_) {});
    _process.exitCode.then(
      (code) => _failAll(StateError('zot bridge exited with code $code')),
    );
  }

  final Process _process;
  late final StreamSubscription<String> _stdoutSubscription;
  late final StreamSubscription<List<int>> _stderrSubscription;
  final _pending = <int, Completer<dynamic>>{};
  final _events = StreamController<Map<String, dynamic>>.broadcast();
  var _nextId = 1;
  var _closed = false;

  static Future<ZotDesktopClient> start(String binaryPath) async =>
      ZotDesktopClient._(await Process.start(binaryPath, const []));

  Future<ZotDesktopSession> createSession(ZotSessionOptions options) async {
    final id = '${DateTime.now().microsecondsSinceEpoch}-${_nextId}';
    await _call('create_session', {
      'session_id': id,
      'provider': options.provider,
      'api_key': options.apiKey,
      'access_token': options.accessToken,
      'account_id': options.accountId,
      'model': options.model,
      'system_prompt': options.systemPrompt,
    });
    return ZotDesktopSession._(this, id);
  }

  Future<dynamic> _call(String method, Map<String, Object?> arguments) {
    if (_closed) {
      return Future.error(StateError('zot bridge is closed'));
    }
    final id = _nextId++;
    final completer = Completer<dynamic>();
    _pending[id] = completer;
    _process.stdin
        .writeln(jsonEncode({'id': id, 'method': method, ...arguments}));
    return completer.future;
  }

  void _receive(String line) {
    final dynamic decoded;
    try {
      decoded = jsonDecode(line);
    } on FormatException {
      return;
    }
    if (decoded is! Map<String, dynamic>) return;
    final value = decoded;
    final id = value['id'];
    if (id is int) {
      final completer = _pending.remove(id);
      if (value['error'] != null) {
        completer?.completeError(StateError(value['error'] as String));
      } else {
        completer?.complete(value['result']);
      }
    } else if (!_events.isClosed) {
      _events.add(value);
    }
  }

  void _failAll(Object error) {
    _closed = true;
    for (final completer in _pending.values) {
      completer.completeError(error);
    }
    _pending.clear();
  }

  Future<void> close() async {
    if (_events.isClosed) return;
    _closed = true;
    _failAll(StateError('zot bridge is closed'));
    await _process.stdin.close();
    try {
      await _process.exitCode.timeout(const Duration(seconds: 2));
    } on TimeoutException {
      _process.kill();
      await _process.exitCode;
    }
    await _stdoutSubscription.cancel();
    await _stderrSubscription.cancel();
    await _events.close();
  }
}

class ZotDesktopSession {
  ZotDesktopSession._(this._client, this.id);
  final ZotDesktopClient _client;
  final String id;

  Stream<ZotEvent> get events => _client._events.stream
          .where((value) => value['session_id'] == id)
          .map((value) {
        final type = value['event'] as String;
        final payload = Map<String, dynamic>.from(value['payload'] as Map);
        return ZotEvent(type, {'type': type, ...payload});
      });
  Future<void> prompt(String message) async =>
      _client._call('prompt', {'session_id': id, 'message': message});
  Future<void> abort() async => _client._call('abort', {'session_id': id});
  Future<String> exportHistory() async {
    final result =
        await _client._call('export_history', {'session_id': id}) as Map;
    return result['history'] as String;
  }

  Future<void> importHistory(String history) async =>
      _client._call('import_history', {'session_id': id, 'history': history});
  Future<void> close() async =>
      _client._call('close_session', {'session_id': id});
}

class ZotNative {
  static const _methods = MethodChannel('dev.zot/methods');
  static final Stream<Map<String, dynamic>> _events =
      const EventChannel('dev.zot/events').receiveBroadcastStream().map(
            (value) => Map<String, dynamic>.from(value as Map),
          );
  static Future<ZotSession> createSession(ZotSessionOptions options) async =>
      ZotSession._(
        (await _methods.invokeMethod<String>(
          'createSession',
          options.toMap(),
        ))!,
      );
  static Future<String> extractOpenAIAccountId(String token) async =>
      (await _methods.invokeMethod<String>('extractOpenAIAccountId', {
        'idToken': token,
      })) ??
      '';
}
