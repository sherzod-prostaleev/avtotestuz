import 'package:avtotest_app/shared/widgets/countdown_timer.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders "1:29" for 89 remaining seconds', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: CountdownTimer(
          remaining: Duration(seconds: 89),
          total: Duration(minutes: 20),
        ),
      ),
    );

    expect(find.text('1:29'), findsOneWidget);
  });

  testWidgets('renders "0:05" for 5 remaining seconds', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: CountdownTimer(
          remaining: Duration(seconds: 5),
          total: Duration(minutes: 20),
        ),
      ),
    );

    expect(find.text('0:05'), findsOneWidget);
  });

  testWidgets('renders "1:00:00" once past an hour', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: CountdownTimer(
          remaining: Duration(hours: 1),
          total: Duration(hours: 2),
        ),
      ),
    );

    expect(find.text('1:00:00'), findsOneWidget);
  });

  testWidgets('shows a low-time warning icon when remaining time is low', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: CountdownTimer(
          remaining: Duration(seconds: 30),
          total: Duration(minutes: 20),
        ),
      ),
    );

    expect(find.byKey(const ValueKey('countdown-timer-low-icon')), findsOneWidget);
  });

  testWidgets('does not show the low-time warning icon with plenty of time left', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: CountdownTimer(
          remaining: Duration(minutes: 15),
          total: Duration(minutes: 20),
        ),
      ),
    );

    expect(find.byKey(const ValueKey('countdown-timer-low-icon')), findsNothing);
  });
}
