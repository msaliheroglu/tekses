import 'package:flutter/material.dart';

import 'ui/join_screen.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(const TekSesApp());
}

class TekSesApp extends StatelessWidget {
  const TekSesApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'TekSes',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        brightness: Brightness.dark,
        scaffoldBackgroundColor: Colors.black,
        colorScheme: ColorScheme.fromSeed(
          seedColor: const Color(0xFFFF2A2A),
          brightness: Brightness.dark,
        ),
        useMaterial3: true,
      ),
      home: const JoinScreen(),
    );
  }
}
