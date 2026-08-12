import Flutter
import UIKit
import Zot

public final class ZotNativePlugin: NSObject, FlutterPlugin, FlutterStreamHandler {
    private var sessions: [String: ZotSession] = [:]
    private var sink: FlutterEventSink?

    public static func register(with registrar: FlutterPluginRegistrar) {
        let plugin = ZotNativePlugin()
        registrar.addMethodCallDelegate(plugin, channel: FlutterMethodChannel(name: "dev.zot/methods", binaryMessenger: registrar.messenger()))
        FlutterEventChannel(name: "dev.zot/events", binaryMessenger: registrar.messenger()).setStreamHandler(plugin)
    }

    public func onListen(withArguments arguments: Any?, eventSink events: @escaping FlutterEventSink) -> FlutterError? { sink = events; return nil }
    public func onCancel(withArguments arguments: Any?) -> FlutterError? { sink = nil; return nil }

    public func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
        let args = call.arguments as? [String: Any] ?? [:]
        switch call.method {
        case "createSession":
            var error: NSError?
            let provider = args["provider"] as? String ?? "anthropic"
            let session: ZotSession?
            if let token = args["accessToken"] as? String {
                session = ZotNewSessionWithOAuth(provider, token, args["accountId"] as? String ?? "", args["model"] as? String ?? "", args["systemPrompt"] as? String ?? "", &error)
            } else {
                session = ZotNewSession(provider, args["apiKey"] as? String ?? "", args["model"] as? String ?? "", args["systemPrompt"] as? String ?? "", &error)
            }
            guard let session else { result(FlutterError(code: "zot_session", message: error?.localizedDescription, details: nil)); return }
            let id = UUID().uuidString; sessions[id] = session; result(id)
        case "prompt":
            let id = args["sessionId"] as! String
            guard let session = sessions[id] else { result(FlutterError(code: "zot_session", message: "Unknown session", details: nil)); return }
            result(nil)
            DispatchQueue.global(qos: .userInitiated).async { session.prompt(args["message"] as! String, stream: FlutterZotStream(id: id, sink: self.sink)) }
        case "abort": sessions[args["sessionId"] as! String]?.abort(); result(nil)
        case "closeSession": sessions.removeValue(forKey: args["sessionId"] as! String)?.abort(); result(nil)
        case "exportHistory":
            guard let session = sessions[args["sessionId"] as! String] else { result(FlutterError(code: "zot_session", message: "Unknown session", details: nil)); return }
            DispatchQueue.global(qos: .userInitiated).async { let history = session.exportHistory(); DispatchQueue.main.async { result(history) } }
        case "importHistory":
            guard let session = sessions[args["sessionId"] as! String] else { result(FlutterError(code: "zot_session", message: "Unknown session", details: nil)); return }
            DispatchQueue.global(qos: .userInitiated).async {
                do { try session.importHistory(args["history"] as! String); DispatchQueue.main.async { result(nil) } }
                catch { DispatchQueue.main.async { result(FlutterError(code: "zot_history", message: error.localizedDescription, details: nil)) } }
            }
        case "extractOpenAIAccountId": result(ZotExtractOpenAIAccountID(args["idToken"] as! String))
        default: result(FlutterMethodNotImplemented)
        }
    }
}

private final class FlutterZotStream: NSObject, ZotStreamProtocol {
    let id: String; let sink: FlutterEventSink?
    init(id: String, sink: FlutterEventSink?) { self.id = id; self.sink = sink }
    private func send(_ event: [String: Any]) { DispatchQueue.main.async { self.sink?(["sessionId": self.id, "event": event]) } }
    func onText(_ delta: String?) { send(["type": "text", "delta": delta ?? ""]) }
    func onEvent(_ kind: String?, payload: String?) { send(["type": kind ?? "event", "payload": payload ?? "{}"]) }
    func onError(_ message: String?) { send(["type": "error", "message": message ?? "Unknown error"]) }
    func onDone() { send(["type": "done"]) }
}
