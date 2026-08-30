import 'package:torch_light/torch_light.dart';

/// Fener denetimi.
///
/// Faz 0'da torch_light eklentisi kullanılır; Faz 2'de hassas zamanlama için
/// native platform kanallarına (Android CameraManager, iOS AVCaptureDevice)
/// taşınacak. Çağrılar asenkron olduğundan istenen durum saklanır ve
/// üst üste binen geçişler tek kuyruğa indirgenir.
class TorchService {
  bool _available = false;
  bool _isOn = false;
  bool _busy = false;
  bool? _pending;

  bool get available => _available;

  Future<void> init() async {
    try {
      _available = await TorchLight.isTorchAvailable();
    } catch (_) {
      _available = false;
    }
  }

  /// Feneri istenen duruma getirir; hata (ör. kamera meşgul) sessizce
  /// yutulur — gösteri sırasında fener arızası koreografiyi durdurmamalıdır.
  Future<void> set(bool on) async {
    if (!_available || on == _isOn) return;
    if (_busy) {
      _pending = on;
      return;
    }
    _busy = true;
    try {
      if (on) {
        await TorchLight.enableTorch();
      } else {
        await TorchLight.disableTorch();
      }
      _isOn = on;
    } catch (_) {
      // yutulur; bir sonraki geçiş yeniden dener
    } finally {
      _busy = false;
      final pending = _pending;
      _pending = null;
      if (pending != null && pending != _isOn) {
        await set(pending);
      }
    }
  }

  Future<void> off() => set(false);
}
