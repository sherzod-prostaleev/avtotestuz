import 'package:avtotest_app/shared/widgets/answer_option.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  // Real keyboard-simulation tests (tester.sendKeyEvent), NOT a mocked
  // shortcut handler — this is the one test in this file that matters most,
  // per the task brief: F1-F4 must genuinely select the right option through
  // Flutter's real key-event/focus pipeline.
  group('F1-F4 keyboard shortcuts select the right option', () {
    late List<String> selections;

    Future<void> pumpGroup(WidgetTester tester) async {
      selections = [];
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: AnswerOptionsGroup(
              options: [
                AnswerOption(
                  label: 'A',
                  text: 'Variant A',
                  selected: false,
                  onSelect: () => selections.add('A'),
                ),
                AnswerOption(
                  label: 'B',
                  text: 'Variant B',
                  selected: false,
                  onSelect: () => selections.add('B'),
                ),
                AnswerOption(
                  label: 'C',
                  text: 'Variant C',
                  selected: false,
                  onSelect: () => selections.add('C'),
                ),
                AnswerOption(
                  label: 'D',
                  text: 'Variant D',
                  selected: false,
                  onSelect: () => selections.add('D'),
                ),
              ],
            ),
          ),
        ),
      );
      // Let the autofocused Focus node actually take focus before sending
      // key events — otherwise they have nowhere to bubble from.
      await tester.pump();
    }

    testWidgets('F1 selects option A', (tester) async {
      await pumpGroup(tester);

      await tester.sendKeyEvent(LogicalKeyboardKey.f1);
      await tester.pump();

      expect(selections, ['A']);
    });

    testWidgets('F2 selects option B', (tester) async {
      await pumpGroup(tester);

      await tester.sendKeyEvent(LogicalKeyboardKey.f2);
      await tester.pump();

      expect(selections, ['B']);
    });

    testWidgets('F3 selects option C', (tester) async {
      await pumpGroup(tester);

      await tester.sendKeyEvent(LogicalKeyboardKey.f3);
      await tester.pump();

      expect(selections, ['C']);
    });

    testWidgets('F4 selects option D', (tester) async {
      await pumpGroup(tester);

      await tester.sendKeyEvent(LogicalKeyboardKey.f4);
      await tester.pump();

      expect(selections, ['D']);
    });
  });

  testWidgets('renders all four options', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: AnswerOptionsGroup(
            options: [
              AnswerOption(label: 'A', text: 'Variant A', selected: false, onSelect: () {}),
              AnswerOption(label: 'B', text: 'Variant B', selected: false, onSelect: () {}),
              AnswerOption(label: 'C', text: 'Variant C', selected: false, onSelect: () {}),
              AnswerOption(label: 'D', text: 'Variant D', selected: false, onSelect: () {}),
            ],
          ),
        ),
      ),
    );

    expect(find.text('Variant A'), findsOneWidget);
    expect(find.text('Variant B'), findsOneWidget);
    expect(find.text('Variant C'), findsOneWidget);
    expect(find.text('Variant D'), findsOneWidget);
  });
}
