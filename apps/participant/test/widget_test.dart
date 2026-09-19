// Katılım ekranı duman testi. Bu dosyanın depoda olması ayrıca önemlidir:
// `flutter create .` var olan dosyaları atlar; bu olmasaydı komut, var
// olmayan MyApp'e işaret eden varsayılan bir widget_test.dart üretirdi.
import 'package:flutter_test/flutter_test.dart';
import 'package:tekses_participant/main.dart';

void main() {
  testWidgets('katılım ekranı açılır', (tester) async {
    await tester.pumpWidget(const TekSesApp());
    expect(find.text('TekSes'), findsOneWidget);
    expect(find.text('Gösteriye katıl'), findsOneWidget);
    expect(find.text('Katılım kodu'), findsOneWidget);
  });
}
