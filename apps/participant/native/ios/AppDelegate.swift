// TekSes native zamanlanmış ses kanalı (tekses/audio) — iOS.
//
// KURULUM: flutter create sonrasında bu dosyayı şuraya KOPYALAYIN (var olanın
// üzerine): ios/Runner/AppDelegate.swift
//
// Zamanlama: AVAudioPlayer.play(atTime:) ses donanımının kendi saatinde
// planlar; kanal, Dart'ın yolladığı monoton hedefi bu eksene çevirir.
import AVFoundation
import Flutter
import UIKit

@main
@objc class AppDelegate: FlutterAppDelegate {
  private var players: [String: AVAudioPlayer] = [:]

  // ProcessInfo.systemUptime saniyedir; kanal ms eksenini bundan türetir.
  private func uptimeMs() -> Int64 { Int64(ProcessInfo.processInfo.systemUptime * 1000.0) }

  // Konum servosu: pos0Sec, parçanın 0. saniyesinin denk geldiği systemUptime
  // anıdır. Çalma oturduktan sonra (start + ~0,7 sn) gerçek konum beklenenle
  // karşılaştırılır; 80 ms'i aşan sapma hedefe çekilir (en çok 3 deneme —
  // kararsız cihazda sonsuz zıplama olmasın).
  private func schedulePositionServo(id: String, pos0Sec: Double, attempt: Int) {
    if attempt >= 3 { return }
    let now = ProcessInfo.processInfo.systemUptime
    let delay = max(0, pos0Sec - now) + 0.7
    DispatchQueue.main.asyncAfter(deadline: .now() + delay) { [weak self] in
      guard let self = self, let player = self.players[id], player.isPlaying else { return }
      let expected = ProcessInfo.processInfo.systemUptime - pos0Sec
      if expected + 0.5 >= player.duration { return } // parça bitmek üzere
      let err = player.currentTime - expected
      if abs(err) > 0.08 {
        player.currentTime = expected
        self.schedulePositionServo(id: id, pos0Sec: pos0Sec, attempt: attempt + 1)
      }
    }
  }

  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    GeneratedPluginRegistrant.register(with: self)

    try? AVAudioSession.sharedInstance().setCategory(.playback)
    try? AVAudioSession.sharedInstance().setActive(true)

    let controller = window?.rootViewController as! FlutterViewController
    let channel = FlutterMethodChannel(
      name: "tekses/audio", binaryMessenger: controller.binaryMessenger)

    channel.setMethodCallHandler { [weak self] call, result in
      guard let self = self else { return }
      switch call.method {
      case "uptimeNow":
        result(self.uptimeMs())

      case "prepare":
        guard let args = call.arguments as? [String: Any],
              let id = args["id"] as? String,
              let path = args["path"] as? String
        else { result(FlutterError(code: "args", message: "id ve path gerekli", details: nil)); return }
        do {
          let player = try AVAudioPlayer(contentsOf: URL(fileURLWithPath: path))
          player.prepareToPlay() // tamponlama burada; çalma anında iş kalmaz
          self.players[id] = player
          result(nil)
        } catch {
          result(FlutterError(code: "prepare", message: error.localizedDescription, details: nil))
        }

      case "playAt":
        guard let args = call.arguments as? [String: Any],
              let id = args["id"] as? String,
              let uptimeTarget = args["uptimeMs"] as? NSNumber,
              let player = self.players[id]
        else { result(FlutterError(code: "args", message: "bilinmeyen id ya da uptimeMs yok", details: nil)); return }
        // seekMs > 0: geç katılan telefon çoktan başlamış parçaya ortasından
        // girer. Konum süreyi aşıyorsa parça bitmiştir, çalma sessizce atlanır.
        let seekMs = (args["seekMs"] as? NSNumber).map { Double(truncating: $0) } ?? 0.0
        if seekMs > 0 {
          let seekSec = seekMs / 1000.0
          if seekSec >= player.duration { result(nil); return }
          player.currentTime = seekSec
        }
        let delaySec = Double(truncating: uptimeTarget) / 1000.0 - ProcessInfo.processInfo.systemUptime
        if delaySec <= 0 {
          player.play()
        } else {
          player.play(atTime: player.deviceCurrentTime + delaySec)
        }
        // Konum servosu (Android eşleniği): çalma hattı gecikmesi cihazdan
        // cihaza değişir; oturduktan sonra gerçek konum beklenenle
        // karşılaştırılır, sapma 80 ms'i aşarsa hedefe çekilir (en çok 3 kez).
        let pos0Sec = Double(truncating: uptimeTarget) / 1000.0 - seekMs / 1000.0
        self.schedulePositionServo(id: id, pos0Sec: pos0Sec, attempt: 0)
        result(nil)

      case "stopAll":
        for player in self.players.values { player.stop() }
        self.players.removeAll()
        result(nil)

      default:
        result(FlutterMethodNotImplemented)
      }
    }

    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }
}
