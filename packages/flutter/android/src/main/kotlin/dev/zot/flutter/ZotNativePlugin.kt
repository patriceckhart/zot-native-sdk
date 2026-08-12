package dev.zot.flutter

import android.os.Handler
import android.os.Looper
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.EventChannel
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.Executors
import zot.Session
import zot.Stream
import zot.Zot

class ZotNativePlugin : FlutterPlugin, MethodChannel.MethodCallHandler, EventChannel.StreamHandler {
  private val sessions = ConcurrentHashMap<String, Session>()
  private val executor = Executors.newCachedThreadPool()
  private val main = Handler(Looper.getMainLooper())
  private var events: EventChannel.EventSink? = null
  override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
    MethodChannel(binding.binaryMessenger, "dev.zot/methods").setMethodCallHandler(this)
    EventChannel(binding.binaryMessenger, "dev.zot/events").setStreamHandler(this)
  }
  override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) { sessions.values.forEach { it.abort() }; sessions.clear() }
  override fun onListen(arguments: Any?, sink: EventChannel.EventSink) { events = sink }
  override fun onCancel(arguments: Any?) { events = null }

  override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
    try {
      val args = call.arguments as? Map<*, *> ?: emptyMap<String, Any>()
      when (call.method) {
        "createSession" -> {
          val provider = args["provider"] as? String ?: "anthropic"
          val session = if (args["accessToken"] != null)
            Zot.newSessionWithOAuth(provider, args["accessToken"] as String, args["accountId"] as? String ?: "", args["model"] as? String ?: "", args["systemPrompt"] as? String ?: "")
          else Zot.newSession(provider, args["apiKey"] as? String ?: "", args["model"] as? String ?: "", args["systemPrompt"] as? String ?: "")
          val id = UUID.randomUUID().toString(); sessions[id] = session; result.success(id)
        }
        "prompt" -> {
          val id = args["sessionId"] as String
          val session = sessions[id] ?: return result.error("zot_session", "Unknown session", null)
          result.success(null)
          executor.execute { session.prompt(args["message"] as String, FlutterStream(id)) }
        }
        "abort" -> { sessions[args["sessionId"]]?.abort(); result.success(null) }
        "closeSession" -> { sessions.remove(args["sessionId"])?.abort(); result.success(null) }
        "exportHistory" -> {
          val session = sessions[args["sessionId"]] ?: return result.error("zot_session", "Unknown session", null)
          executor.execute { main.post { result.success(session.exportHistory()) } }
        }
        "importHistory" -> {
          val session = sessions[args["sessionId"]] ?: return result.error("zot_session", "Unknown session", null)
          executor.execute {
            try { session.importHistory(args["history"] as String); main.post { result.success(null) } }
            catch (error: Exception) { main.post { result.error("zot_history", error.message, null) } }
          }
        }
        "extractOpenAIAccountId" -> result.success(Zot.extractOpenAIAccountID(args["idToken"] as String))
        else -> result.notImplemented()
      }
    } catch (error: Exception) { result.error("zot_error", error.message, null) }
  }

  inner class FlutterStream(private val id: String) : Stream {
    private fun send(event: Map<String, Any>) { main.post { events?.success(mapOf("sessionId" to id, "event" to event)) } }
    override fun onText(delta: String) = send(mapOf("type" to "text", "delta" to delta))
    override fun onEvent(kind: String, payload: String) = send(mapOf("type" to kind, "payload" to payload))
    override fun onError(message: String) = send(mapOf("type" to "error", "message" to message))
    override fun onDone() = send(mapOf("type" to "done"))
  }
}
