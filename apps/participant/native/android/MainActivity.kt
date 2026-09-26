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
                    // seekMs > 0: geç katılan telefon çoktan başlamış parçaya
                    // ortasından girer. Konum süreyi aşıyorsa parça bitmiştir,
                    // çalma sessizce atlanır.
                    val seekMs = call.argument<Number>("seekMs")?.toLong() ?: 0L
                    val player = if (id != null) players[id] else null
                    if (player == null || uptimeMs == null) {
                        result.error("args", "bilinmeyen id ya da uptimeMs yok", null)
                        return@setMethodCallHandler
                    }
                    if (seekMs > 0 && seekMs >= player.duration) {
                        result.success(null); return@setMethodCallHandler
                    }
                    if (seekMs > 0) {
                        seekPrecise(player, seekMs)
                    }
                    val now = SystemClock.uptimeMillis()
                    if (uptimeMs <= now) player.start()
                    else handler.postAtTime({ player.start() }, uptimeMs)
                    // Konum servosu: start() komutu anında ses BAŞLAMAZ —
                    // kod çözücü/tampon gecikmesi cihazdan cihaza 50-300 ms
                    // değişir ve iki telefon aynı anda başlatılsa bile sabit
                    // bir kayma bırakır (saha bulgusu, 2026-09-26: kayma
                    // m4a'da da sürdü, yani konum tablosu değil hat gecikmesi).
                    // Çalma oturduktan sonra gerçek konum beklenenle
                    // karşılaştırılır; sapma eşiği aşarsa hedefe yeniden
                    // atlanır (bir kez kısa bir sıçrama duyulabilir — kalıcı
                    // kaymadan iyidir).
                    schedulePositionServo(
                        player,
                        pos0Uptime = uptimeMs - seekMs,
                        checkAtUptime = maxOf(uptimeMs, now) + 700, // çalma otursun
                        attempt = 0,
                    )
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

    // Örnek hassasiyetli atlama: API 26+ SEEK_CLOSEST (varsayılan seekTo en
    // yakın senkron kareye atlayıp yüzlerce ms kayabilir).
    private fun seekPrecise(player: MediaPlayer, positionMs: Long) {
        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.O) {
            player.seekTo(positionMs, MediaPlayer.SEEK_CLOSEST)
        } else {
            player.seekTo(positionMs.toInt())
        }
    }

    // Servo eşiği/planı: ilk ölçüm çalma oturduktan sonra (start + ~700 ms),
    // düzeltme sonrası bir doğrulama daha; en çok 3 deneme (kararsız cihazda
    // sonsuz atlama-zıplama olmasın). 80 ms eşiği: bunun altı telefon
    // hoparlörlerinde koro etkisi olarak zaten duyulur ama rahatsız etmez;
    // ürün sözü de akustik birlik değildir (ses PA'dan, telefon ışık/söz).
    private fun schedulePositionServo(
        player: MediaPlayer,
        pos0Uptime: Long,
        checkAtUptime: Long,
        attempt: Int,
    ) {
        if (attempt >= 3) return
        handler.postAtTime({
            // stopAll/yeniden hazırlama sonrası bayat servo koşmasın.
            if (!players.containsValue(player)) return@postAtTime
            try {
                if (!player.isPlaying) return@postAtTime
                val expected = SystemClock.uptimeMillis() - pos0Uptime
                if (expected + 500 >= player.duration) return@postAtTime // parça bitmek üzere
                val errMs = player.currentPosition - expected // + = ileride
                if (errMs < -80 || errMs > 80) {
                    seekPrecise(player, expected)
                    // Doğrulama turu: atlama kendisi de zaman yer; kalan hata
                    // eşiğin altına inene ya da deneme hakkı bitene dek sürer.
                    schedulePositionServo(
                        player, pos0Uptime,
                        checkAtUptime = SystemClock.uptimeMillis() + 700,
                        attempt = attempt + 1,
                    )
                }
            } catch (_: Exception) {
                // yarış (tam o anda durduruldu): servo sessizce vazgeçer
            }
        }, checkAtUptime)
    }

    override fun onDestroy() {
        handler.removeCallbacksAndMessages(null)
        for (player in players.values) player.release()
        players.clear()
        super.onDestroy()
    }
}
