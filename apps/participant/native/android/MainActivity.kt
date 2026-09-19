// TekSes native zamanlanmış ses kanalı (tekses/audio).
//
// KURULUM: flutter create sonrasında bu dosyayı şuraya KOPYALAYIN (var olanın
// üzerine): android/app/src/main/kotlin/app/tekses/tekses_participant/MainActivity.kt
//
// Zamanlama: çalma anını Dart değil, Android'in ana döngüsü tutar —
// Handler.postAtTime SystemClock.uptimeMillis ekseninde çalışır; Dart bu
// ekseni MonoClock'una bir kez eşleyip mutlak hedef yollar (native_audio.dart).
package app.tekses.tekses_participant

import android.media.AudioAttributes
import android.media.MediaPlayer
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    private val players = HashMap<String, MediaPlayer>()
    private val handler = Handler(Looper.getMainLooper())

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        MethodChannel(
            flutterEngine.dartExecutor.binaryMessenger,
            "tekses/audio"
        ).setMethodCallHandler { call, result ->
            when (call.method) {
                "uptimeNow" -> result.success(SystemClock.uptimeMillis())

                "prepare" -> {
                    val id = call.argument<String>("id")
                    val path = call.argument<String>("path")
                    if (id == null || path == null) {
                        result.error("args", "id ve path gerekli", null); return@setMethodCallHandler
                    }
                    try {
                        players.remove(id)?.release()
                        val player = MediaPlayer()
                        player.setAudioAttributes(
                            AudioAttributes.Builder()
                                .setUsage(AudioAttributes.USAGE_MEDIA)
                                .setContentType(AudioAttributes.CONTENT_TYPE_MUSIC)
                                .build()
                        )
                        player.setDataSource(path)
                        player.prepare() // kod çözme burada; çalma anında iş kalmaz
                        players[id] = player
                        result.success(null)
                    } catch (e: Exception) {
                        result.error("prepare", e.message, null)
                    }
                }

                "playAt" -> {
                    val id = call.argument<String>("id")
                    val uptimeMs = call.argument<Number>("uptimeMs")?.toLong()
                    val player = if (id != null) players[id] else null
                    if (player == null || uptimeMs == null) {
                        result.error("args", "bilinmeyen id ya da uptimeMs yok", null)
                        return@setMethodCallHandler
                    }
                    val now = SystemClock.uptimeMillis()
                    if (uptimeMs <= now) player.start()
                    else handler.postAtTime({ player.start() }, uptimeMs)
                    result.success(null)
                }

                "stopAll" -> {
                    handler.removeCallbacksAndMessages(null)
                    for (player in players.values) {
                        try { player.stop() } catch (_: Exception) {}
                        player.release()
                    }
                    players.clear()
                    result.success(null)
                }

                else -> result.notImplemented()
            }
        }
    }

    override fun onDestroy() {
        handler.removeCallbacksAndMessages(null)
        for (player in players.values) player.release()
        players.clear()
        super.onDestroy()
    }
}
