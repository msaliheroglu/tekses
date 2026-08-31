import 'package:flutter/material.dart';

import '../core/package_store.dart';
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
  String? _error;
  bool _busy = false;

  @override
  void dispose() {
    _gatewayController.dispose();
    _controlController.dispose();
    _codeController.dispose();
    super.dispose();
  }

  Future<void> _join() async {
    final gatewayUri = Uri.tryParse(_gatewayController.text.trim());
    if (gatewayUri == null || (gatewayUri.scheme != 'ws' && gatewayUri.scheme != 'wss')) {
      setState(() => _error = 'Gateway adresi ws:// veya wss:// ile başlamalı');
      return;
    }
    final code = _codeController.text.trim().toUpperCase();

    JoinInfo? joinInfo;
    if (code.isNotEmpty) {
      final controlUri = Uri.tryParse(_controlController.text.trim());
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

    if (!mounted) return;
    setState(() => _error = null);
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => ShowScreen(
          serverUri: gatewayUri,
          joinInfo: joinInfo,
          joinCode: code,
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
                'sonuna kadar açın. Yanıp sönen ışık içerir.',
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
