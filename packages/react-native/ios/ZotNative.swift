import Foundation
import React
import Zot

@objc(ZotNative)
final class ZotNative: RCTEventEmitter {
    private var sessions: [String: ZotSession] = [:]
    private var hasListeners = false

    override static func requiresMainQueueSetup() -> Bool { false }
    override func supportedEvents() -> [String] { ["ZotEvent"] }
    override func startObserving() { hasListeners = true }
    override func stopObserving() { hasListeners = false }

    @objc func createSession(_ options: NSDictionary, resolver resolve: RCTPromiseResolveBlock, rejecter reject: RCTPromiseRejectBlock) {
        var error: NSError?
        let provider = options["provider"] as? String ?? "anthropic"
        let model = options["model"] as? String ?? ""
        let prompt = options["systemPrompt"] as? String ?? ""
        let session: ZotSession?
        if let token = options["accessToken"] as? String {
            session = ZotNewSessionWithOAuth(provider, token, options["accountId"] as? String ?? "", model, prompt, &error)
        } else {
            session = ZotNewSession(provider, options["apiKey"] as? String ?? "", model, prompt, &error)
        }
        guard let session else {
            reject("zot_session", error?.localizedDescription ?? "Could not create session", error)
            return
        }
        let id = UUID().uuidString
        sessions[id] = session
        resolve(id)
    }

    @objc func prompt(_ sessionId: String, message: String, resolver resolve: @escaping RCTPromiseResolveBlock, rejecter reject: @escaping RCTPromiseRejectBlock) {
        guard let session = sessions[sessionId] else {
            reject("zot_session", "Unknown session", nil)
            return
        }
        resolve(nil)
        DispatchQueue.global(qos: .userInitiated).async {
            session.prompt(message, stream: RNStream { [weak self] event in self?.emit(sessionId, event) })
        }
    }

    @objc func abort(_ sessionId: String, resolver resolve: RCTPromiseResolveBlock, rejecter reject: RCTPromiseRejectBlock) {
        sessions[sessionId]?.abort()
        resolve(nil)
    }

    @objc func closeSession(_ sessionId: String, resolver resolve: RCTPromiseResolveBlock, rejecter reject: RCTPromiseRejectBlock) {
        sessions.removeValue(forKey: sessionId)?.abort()
        resolve(nil)
    }

    @objc func exportHistory(_ sessionId: String, resolver resolve: RCTPromiseResolveBlock, rejecter reject: RCTPromiseRejectBlock) {
        guard let session = sessions[sessionId] else { reject("zot_session", "Unknown session", nil); return }
        DispatchQueue.global(qos: .userInitiated).async { resolve(session.exportHistory()) }
    }

    @objc func importHistory(_ sessionId: String, history: String, resolver resolve: RCTPromiseResolveBlock, rejecter reject: RCTPromiseRejectBlock) {
        guard let session = sessions[sessionId] else { reject("zot_session", "Unknown session", nil); return }
        DispatchQueue.global(qos: .userInitiated).async {
            do { try session.importHistory(history); resolve(nil) }
            catch { reject("zot_history", error.localizedDescription, error) }
        }
    }

    @objc func extractOpenAIAccountId(_ idToken: String, resolver resolve: RCTPromiseResolveBlock, rejecter reject: RCTPromiseRejectBlock) {
        resolve(ZotExtractOpenAIAccountID(idToken))
    }

    private func emit(_ sessionId: String, _ event: [String: Any]) {
        DispatchQueue.main.async { [weak self] in
            guard let self, self.hasListeners else { return }
            self.sendEvent(withName: "ZotEvent", body: ["sessionId": sessionId, "event": event])
        }
    }
}

private final class RNStream: NSObject, ZotStreamProtocol {
    let emit: ([String: Any]) -> Void
    init(emit: @escaping ([String: Any]) -> Void) { self.emit = emit }
    func onText(_ delta: String?) { emit(["type": "text", "delta": delta ?? ""]) }
    func onEvent(_ kind: String?, payload: String?) {
        let data = (payload ?? "{}").data(using: .utf8) ?? Data()
        let value = (try? JSONSerialization.jsonObject(with: data)) ?? [:]
        emit(["type": kind ?? "event", "payload": value])
    }
    func onError(_ message: String?) { emit(["type": "error", "message": message ?? "Unknown error"]) }
    func onDone() { emit(["type": "done"]) }
}
