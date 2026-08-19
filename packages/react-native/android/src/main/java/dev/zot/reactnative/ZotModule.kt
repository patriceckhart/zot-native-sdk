package dev.zot.reactnative

import com.facebook.react.bridge.*
import com.facebook.react.modules.core.DeviceEventManagerModule
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.Executors
import zot.Session
import zot.Stream
import zot.Zot

class ZotModule(private val context: ReactApplicationContext) : ReactContextBaseJavaModule(context) {
  private val sessions = ConcurrentHashMap<String, Session>()
  private val executor = Executors.newCachedThreadPool()
  override fun getName() = "ZotNative"

  @ReactMethod fun createSession(options: ReadableMap, promise: Promise) {
    try {
      val provider = options.getString("provider") ?: "anthropic"
      val model = if (options.hasKey("model")) options.getString("model") ?: "" else ""
      val prompt = if (options.hasKey("systemPrompt")) options.getString("systemPrompt") ?: "" else ""
      val session = if (options.hasKey("accessToken"))
        Zot.newSessionWithOAuth(provider, options.getString("accessToken") ?: "", if (options.hasKey("accountId")) options.getString("accountId") ?: "" else "", model, prompt)
      else Zot.newSession(provider, options.getString("apiKey") ?: "", model, prompt)
      val id = UUID.randomUUID().toString()
      sessions[id] = session
      promise.resolve(id)
    } catch (error: Exception) { promise.reject("zot_session", error) }
  }

  @ReactMethod fun prompt(sessionId: String, message: String, promise: Promise) {
    val session = sessions[sessionId] ?: return promise.reject("zot_session", "Unknown session")
    promise.resolve(null)
    executor.execute { session.prompt(message, EventStream(sessionId)) }
  }
  @ReactMethod fun abort(sessionId: String, promise: Promise) { sessions[sessionId]?.abort(); promise.resolve(null) }
  @ReactMethod fun closeSession(sessionId: String, promise: Promise) { sessions.remove(sessionId)?.abort(); promise.resolve(null) }
  @ReactMethod fun exportHistory(sessionId: String, promise: Promise) {
    val session = sessions[sessionId] ?: return promise.reject("zot_session", "Unknown session")
    executor.execute { promise.resolve(session.exportHistory()) }
  }
  @ReactMethod fun importHistory(sessionId: String, history: String, promise: Promise) {
    val session = sessions[sessionId] ?: return promise.reject("zot_session", "Unknown session")
    executor.execute { try { session.importHistory(history); promise.resolve(null) } catch (error: Exception) { promise.reject("zot_history", error) } }
  }
  @ReactMethod fun extractOpenAIAccountId(token: String, promise: Promise) { promise.resolve(Zot.extractOpenAIAccountID(token)) }
  @ReactMethod fun addListener(name: String) {}
  @ReactMethod fun removeListeners(count: Int) {}

  override fun invalidate() {
    sessions.values.forEach { it.abort() }
    sessions.clear()
    executor.shutdownNow()
    super.invalidate()
  }

  private fun emit(sessionId: String, event: WritableMap) {
    context.runOnUiQueueThread {
      val body = Arguments.createMap().apply { putString("sessionId", sessionId); putMap("event", event) }
      context.getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter::class.java).emit("ZotEvent", body)
    }
  }

  inner class EventStream(private val id: String) : Stream {
    override fun onText(delta: String) = emit(id, Arguments.createMap().apply { putString("type", "text"); putString("delta", delta) })
    override fun onEvent(kind: String, payload: String) = emit(id, Arguments.createMap().apply { putString("type", kind); putString("payload", payload) })
    override fun onError(message: String) = emit(id, Arguments.createMap().apply { putString("type", "error"); putString("message", message) })
    override fun onDone() = emit(id, Arguments.createMap().apply { putString("type", "done") })
  }
}
