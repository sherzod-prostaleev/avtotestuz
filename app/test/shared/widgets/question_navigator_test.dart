import 'package:avtotest_app/shared/widgets/question_navigator.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders one cell per question, 1-based labels', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: QuestionNavigator(
          total: 20,
          currentIndex: 0,
          answeredIndices: const {},
          onJump: (_) {},
        ),
      ),
    );

    expect(find.text('1'), findsOneWidget);
    expect(find.text('20'), findsOneWidget);
  });

  testWidgets('tapping a cell calls onJump with its (zero-based) index', (tester) async {
    int? jumpedTo;
    await tester.pumpWidget(
      MaterialApp(
        home: QuestionNavigator(
          total: 20,
          currentIndex: 0,
          answeredIndices: const {},
          onJump: (index) => jumpedTo = index,
        ),
      ),
    );

    await tester.tap(find.text('5'));
    await tester.pump();

    expect(jumpedTo, 4);
  });

  testWidgets('current/answered/unanswered cells render with distinct colors', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: QuestionNavigator(
          total: 5,
          currentIndex: 2,
          answeredIndices: const {0, 1},
          onJump: (_) {},
        ),
      ),
    );

    Color? colorOf(int index) => tester
        .widget<Material>(find.byKey(ValueKey('question-navigator-cell-$index')))
        .color;

    final answered = colorOf(0);
    final current = colorOf(2);
    final unanswered = colorOf(4);

    expect({answered, current, unanswered}.length, 3, reason:
        'answered, current, and unanswered cells must all render distinctly');
  });
}
