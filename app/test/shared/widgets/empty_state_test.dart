import 'package:avtotest_app/shared/widgets/empty_state.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders the message and no retry button when onRetry is '
      'omitted', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: EmptyState(message: 'Hech narsa topilmadi')),
    );

    expect(find.text('Hech narsa topilmadi'), findsOneWidget);
    expect(find.byType(ElevatedButton), findsNothing);
  });

  testWidgets('renders a retry button with the given label and fires '
      'onRetry when tapped', (tester) async {
    var retried = false;
    await tester.pumpWidget(
      MaterialApp(
        home: EmptyState(
          message: 'Xatolik yuz berdi',
          onRetry: () => retried = true,
          retryLabel: 'Qayta urinish',
        ),
      ),
    );

    expect(find.text('Qayta urinish'), findsOneWidget);

    await tester.tap(find.text('Qayta urinish'));
    await tester.pump();

    expect(retried, isTrue);
  });
}
