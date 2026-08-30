import 'package:flutter/material.dart';

import 'show_screen.dart';

/// Faz 0 katılım ekranı: gateway adresi girilir ve gösteriye bağlanılır.
/// Faz 1'de yerini katılım kodu + QR akışına bırakacak.
class JoinScreen extends StatefulWidget {
  const JoinScreen({super.key});

  @override
  State<JoinScreen> createState() => _JoinScreenState();
}

class _JoinScreenState extends State<JoinScreen> {
  final _urlController =
      TextEditingController(text: 'ws://192.168.1.10:8080/ws');
  String? _error;

  @override
  void dispose() {
    _urlController.dispose();
    super.dispose();
  }

  void _join() {
    final text = _urlController.text.trim();
    final uri = Uri.tryParse(text);
    if (uri == null || (uri.scheme != 'ws' && uri.scheme != 'wss')) {
      setState(() => _error = 'Adres ws:// veya wss:// ile başlamalı');
      return;
    }
    setState(() => _error = null);
    Navigator.of(context).push(
      MaterialPageRoute(builder: (_) => ShowScreen(serverUri: uri)),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Text(
                'TekSes',
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: 42, fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 8),
              Text(
                'Faz 0 — senkron denemesi',
                textAlign: TextAlign.center,
                style: TextStyle(color: Colors.grey.shade400),
              ),
              const SizedBox(height: 32),
              TextField(
                controller: _urlController,
                keyboardType: TextInputType.url,
                autocorrect: false,
                decoration: InputDecoration(
                  labelText: 'Gateway adresi',
                  hintText: 'ws://<sunucu-ip>:8080/ws',
                  errorText: _error,
                  border: const OutlineInputBorder(),
                ),
                onSubmitted: (_) => _join(),
              ),
              const SizedBox(height: 16),
              FilledButton(
                onPressed: _join,
                style: FilledButton.styleFrom(
                  padding: const EdgeInsets.symmetric(vertical: 16),
                ),
                child: const Text('Gösteriye katıl', style: TextStyle(fontSize: 18)),
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
