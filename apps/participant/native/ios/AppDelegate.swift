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
        let delaySec = Double(truncating: uptimeTarget) / 1000.0 - ProcessInfo.processInfo.systemUptime
        if delaySec <= 0 {
          player.play()
        } else {
          player.play(atTime: player.deviceCurrentTime + delaySec)
        }
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
