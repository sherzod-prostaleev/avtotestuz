import 'package:avtotest_app/shared/widgets/primary_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders its label and fires onPressed when tapped', (
    tester,
  ) async {
    var tapped = false;
    await tester.pumpWidget(
      MaterialApp(
        home: PrimaryButton(label: 'Davom etish', onPressed: () => tapped = true),
      ),
    );

    expect(find.text('Davom etish'), findsOneWidget);

    await tester.tap(find.byType(PrimaryButton));
    await tester.pump();

    expect(tapped, isTrue);
  });

  testWidgets('shows a spinner instead of the label while loading, and '
      'ignores taps', (tester) async {
    var tapped = false;
    await tester.pumpWidget(
      MaterialApp(
        home: PrimaryButton(
          label: 'Davom etish',
          onPressed: () => tapped = true,
          loading: true,
        ),
      ),
    );

    expect(find.text('Davom etish'), findsNothing);
    expect(find.byType(CircularProgressIndicator), findsOneWidget);

    await tester.tap(find.byType(PrimaryButton));
    await tester.pump();

    expect(tapped, isFalse);
  });

  testWidgets('a null onPressed disables the button', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: PrimaryButton(label: 'Davom etish', onPressed: null),
      ),
    );

    final button = tester.widget<ElevatedButton>(
      find.byType(ElevatedButton),
    );
    expect(button.onPressed, isNull);
  });
}
