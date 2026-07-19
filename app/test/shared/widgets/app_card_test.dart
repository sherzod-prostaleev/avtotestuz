import 'package:avtotest_app/shared/widgets/app_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders its child', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: AppCard(child: Text('Hello'))),
    );

    expect(find.text('Hello'), findsOneWidget);
  });

  testWidgets('fires onTap when enabled and tapped', (tester) async {
    var tapped = false;
    await tester.pumpWidget(
      MaterialApp(
        home: AppCard(onTap: () => tapped = true, child: const Text('Tap me')),
      ),
    );

    await tester.tap(find.text('Tap me'));
    await tester.pump();

    expect(tapped, isTrue);
  });

  testWidgets('enabled: false renders at reduced opacity and ignores taps '
      'entirely (an honest placeholder, not a dead-looking live button)', (
    tester,
  ) async {
    var tapped = false;
    await tester.pumpWidget(
      MaterialApp(
        home: AppCard(
          enabled: false,
          onTap: () => tapped = true,
          child: const Text('Tez orada'),
        ),
      ),
    );

    expect(find.byType(InkWell), findsNothing);
    final opacity = tester.widget<Opacity>(find.byType(Opacity));
    expect(opacity.opacity, lessThan(1));

    await tester.tap(find.text('Tez orada'), warnIfMissed: false);
    await tester.pump();

    expect(tapped, isFalse);
  });
}
