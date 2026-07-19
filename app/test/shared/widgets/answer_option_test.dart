import 'package:avtotest_app/shared/widgets/answer_option.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Widget wrap(Widget child) => MaterialApp(home: Scaffold(body: child));

  testWidgets('calls onSelect when tapped', (tester) async {
    var selected = false;
    await tester.pumpWidget(
      wrap(
        AnswerOption(
          label: 'A',
          text: 'Variant A',
          selected: false,
          onSelect: () => selected = true,
        ),
      ),
    );

    await tester.tap(find.text('Variant A'));
    await tester.pump();

    expect(selected, isTrue);
  });

  testWidgets('renders selected vs unselected with a distinct border', (tester) async {
    await tester.pumpWidget(
      wrap(
        AnswerOption(label: 'A', text: 'Variant A', selected: false, onSelect: () {}),
      ),
    );
    final unselected = tester.widget<Material>(find.byKey(const ValueKey('answer-option-A')));
    final unselectedShape = unselected.shape! as RoundedRectangleBorder;

    await tester.pumpWidget(
      wrap(
        AnswerOption(label: 'A', text: 'Variant A', selected: true, onSelect: () {}),
      ),
    );
    final selected = tester.widget<Material>(find.byKey(const ValueKey('answer-option-A')));
    final selectedShape = selected.shape! as RoundedRectangleBorder;

    expect(selectedShape.side.width, greaterThan(unselectedShape.side.width));
    expect(selected.color, isNot(equals(unselected.color)));
  });

  testWidgets('renders each AnswerFeedback state distinctly', (tester) async {
    Future<Color?> colorFor(AnswerFeedback? feedback) async {
      await tester.pumpWidget(
        wrap(
          AnswerOption(
            label: 'A',
            text: 'Variant A',
            selected: false,
            feedback: feedback,
            onSelect: () {},
          ),
        ),
      );
      final material = tester.widget<Material>(find.byKey(const ValueKey('answer-option-A')));
      return material.color;
    }

    final none = await colorFor(null);
    final correct = await colorFor(AnswerFeedback.correct);
    final incorrect = await colorFor(AnswerFeedback.incorrect);
    final theCorrectOne = await colorFor(AnswerFeedback.theCorrectOne);

    final colors = {none, correct, incorrect, theCorrectOne};
    expect(colors.length, 4, reason: 'all four feedback states must render distinctly');
  });

  testWidgets('shows a check icon for correct feedback and a cancel icon for incorrect', (
    tester,
  ) async {
    await tester.pumpWidget(
      wrap(
        AnswerOption(
          label: 'A',
          text: 'Variant A',
          selected: false,
          feedback: AnswerFeedback.correct,
          onSelect: () {},
        ),
      ),
    );
    expect(find.byIcon(Icons.check_circle), findsOneWidget);

    await tester.pumpWidget(
      wrap(
        AnswerOption(
          label: 'A',
          text: 'Variant A',
          selected: false,
          feedback: AnswerFeedback.incorrect,
          onSelect: () {},
        ),
      ),
    );
    expect(find.byIcon(Icons.cancel), findsOneWidget);
  });
}
