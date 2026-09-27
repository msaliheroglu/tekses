import 'package:flutter/material.dart';

import '../core/package_store.dart';
import '../core/venue.dart';
import 'show_screen.dart';

/// Katılım ekranı.
///
/// İki yol vardır:
/// - **Katılım kodu ile** (Faz 1): kod control-api'den çözülür, gösteri
///   paketi indirilip SHA-256 ile doğrulanır, sonra gateway'e bağlanılır.
/// - **Kodsuz** (Faz 0 denemesi): doğrudan gateway'in varsayılan odasına
///   girilir.
/// QR ile otomatik doldurma sonraki yineleme.
class JoinScreen extends StatefulWidget {
  const JoinScreen({super.key});

  @override
  State<JoinScreen> createState() => _JoinScreenState();
}

class _JoinScreenState extends State<JoinScreen> {
  final _gatewayController = TextEditingController(text: 'ws://192.168.1.10:8080/ws');
  final _controlController = TextEditingController(text: 'http://192.168.1.10:8090');
  final _codeController = TextEditingController();
  final _seatController = TextEditingController();
  String? _error;
  bool _busy = false;

  @override
  void dispose() {
    _gatewayController.dispose();
    _controlController.dispose();
    _codeController.dispose();
    _seatController.dispose();
    super.dispose();
  }

  Future<void> _join() async {
    final gatewayUri = Uri.tryParse(_gatewayController.text.trim());
    if (gatewayUri == null || (gatewayUri.scheme != 'ws' && gatewayUri.scheme != 'wss')) {
      setState(() => _error = 'Gateway adresi ws:// veya wss:// ile başlamalı');
      return;
    }
    final code = _codeController.text.trim().toUpperCase();

    // Koltuk kimliği (F4.1): biçim burada, mekân planına uygunluk paket
    // indikten sonra denetlenir. Harf katlanmaz — plandaki kimlikle birebir
    // yazılmalı (bilet/QR zaten doğru biçimi taşır).
    final seatText = _seatController.text.trim();
    SeatRef? seat;
    if (seatText.isNotEmpty) {
      seat = SeatRef.parse(seatText);
      if (seat == null) {
        setState(() =>
            _error = 'Koltuk BLOK-SIRA-KOLTUK biçiminde olmalı (ör. A-12-5)');
        return;
      }
    }

    JoinInfo? joinInfo;
    Uri? controlUri;
    if (code.isNotEmpty) {
      controlUri = Uri.tryParse(_controlController.text.trim());
      if (controlUri == null || (controlUri.scheme != 'http' && controlUri.scheme != 'https')) {
        setState(() => _error = 'Control adresi http:// veya https:// ile başlamalı');
        return;
      }
      setState(() {
        _error = null;
        _busy = true;
      });
      try {
        // Paket burada, odaya girerken iner; kue anında ağ gerekmez.
        joinInfo = await PackageStore().join(controlUri, code);
      } catch (err) {
        setState(() {
          _busy = false;
          _error = err is PackageStoreException ? err.message : 'Katılım başarısız: $err';
        });
        return;
      }
      setState(() => _busy = false);
    }

    // Girilen koltuk mekân planında yoksa yanlış yazım büyük olasılıkla:
    // sessizce konumsuz oynamak yerine burada durdur, kullanıcı düzeltsin.
    final venue = joinInfo?.manifest?.venue;
    if (seat != null && venue != null && venue.resolve(seat) == null) {
      setState(() => _error =
          'Koltuk $seat mekân planında yok — biletteki blok/sıra/koltuğu kontrol edin');
      return;
    }

    if (!mounted) return;
    setState(() => _error = null);
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => ShowScreen(
          serverUri: gatewayUri,
          joinInfo: joinInfo,
          joinCode: code,
          // Canlı paket yenileme (show_activated) için gerekli.
          controlUri: controlUri,
          seat: seat,
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: 32),
              const Text(
                'TekSes',
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: 42, fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 32),
              TextField(
                controller: _codeController,
                textCapitalization: TextCapitalization.characters,
                autocorrect: false,
                decoration: const InputDecoration(
                  labelText: 'Katılım kodu',
                  hintText: 'ör. ABC234 (Faz 0 denemesi için boş bırakın)',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _seatController,
                autocorrect: false,
                decoration: const InputDecoration(
                  labelText: 'Koltuk (biletinizde varsa)',
                  hintText: 'BLOK-SIRA-KOLTUK, ör. A-12-5',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _gatewayController,
                keyboardType: TextInputType.url,
                autocorrect: false,
                decoration: const InputDecoration(
                  labelText: 'Gateway adresi',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _controlController,
                keyboardType: TextInputType.url,
                autocorrect: false,
                decoration: const InputDecoration(
                  labelText: 'Control API adresi (kodla katılım için)',
                  border: OutlineInputBorder(),
                ),
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(_error!, style: const TextStyle(color: Colors.redAccent)),
              ],
              const SizedBox(height: 16),
              FilledButton(
                onPressed: _busy ? null : _join,
                style: FilledButton.styleFrom(
                  padding: const EdgeInsets.symmetric(vertical: 16),
                ),
                child: Text(
                  _busy ? 'Paket indiriliyor…' : 'Gösteriye katıl',
                  style: const TextStyle(fontSize: 18),
                ),
              ),
              const SizedBox(height: 24),
              Text(
                'Deneme sırasında ekran açık kalır ve parlaklığı elle '
                'sonuna kadar açın. Gösteri YANIP SÖNEN IŞIK içerebilir; '
                'ışığa duyarlıysanız gösteri ekranının sol altındaki '
                '"ışığa duyarlı mod"u açın (yanıp sönme sabit ışığa döner).',
                textAlign: TextAlign.center,
                style: TextStyle(color: Colors.grey.shade500, fontSize: 12),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
