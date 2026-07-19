import 'package:avtotest_app/features/content/domain/question.dart';
import 'package:avtotest_app/features/explanation/presentation/explanation_view.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  // Wrapped in a scrollable: `ExplanationView` is a plain (non-scrolling)
  // Column by design, meant to be embedded inside a scrollable parent by
  // whichever screen uses it (session/results screen, a later task) — the
  // test harness's fixed viewport would otherwise overflow when rendering
  // every block type at once, which isn't a real bug in the widget itself.
  Widget wrap(Widget child) =>
      MaterialApp(home: Scaffold(body: SingleChildScrollView(child: child)));

  Color? backgroundOf(WidgetTester tester, Key key) {
    final container = tester.widget<Container>(find.byKey(key));
    return (container.decoration as BoxDecoration?)?.color;
  }

  IconData iconOf(WidgetTester tester, Key key) {
    return tester.widget<Icon>(find.byKey(key)).icon!;
  }

  Color? iconColorOf(WidgetTester tester, Key key) {
    return tester.widget<Icon>(find.byKey(key)).color;
  }

  testWidgets('renders each recognized block type with distinct icon/color',
      (tester) async {
    const blocks = [
      ExplanationBlock(type: 'intro', text: 'Kirish matni'),
      ExplanationBlock(type: 'muhim', text: 'Muhim matn'),
      ExplanationBlock(type: 'eslatma', text: 'Eslatma matni'),
      ExplanationBlock(type: 'ogohlantirish', text: 'Ogohlantirish matni'),
      ExplanationBlock(type: 'maslahat', text: 'Maslahat matni'),
      ExplanationBlock(
        type: 'answer_analysis',
        items: [
          AnswerAnalysisItem(position: 1, correct: false, text: 'Noto\'g\'ri'),
          AnswerAnalysisItem(position: 2, correct: true, text: 'To\'g\'ri'),
        ],
      ),
      ExplanationBlock(type: 'xulosa', text: 'Xulosa matni'),
    ];

    await tester.pumpWidget(wrap(const ExplanationView(blocks: blocks)));

    // All texts render.
    expect(find.text('Kirish matni'), findsOneWidget);
    expect(find.text('Muhim matn'), findsOneWidget);
    expect(find.text('Eslatma matni'), findsOneWidget);
    expect(find.text('Ogohlantirish matni'), findsOneWidget);
    expect(find.text('Maslahat matni'), findsOneWidget);
    expect(find.text('Xulosa matni'), findsOneWidget);

    // Each type has its own icon.
    final types = [
      'intro',
      'muhim',
      'eslatma',
      'ogohlantirish',
      'maslahat',
      'answer_analysis',
      'xulosa',
    ];
    final icons = <IconData>{};
    final backgrounds = <Color?>{};
    for (final type in types) {
      final icon = iconOf(tester, Key('explanation-block-icon-$type'));
      final bg = backgroundOf(tester, Key('explanation-block-$type'));
      icons.add(icon);
      backgrounds.add(bg);
    }
    // Every type gets a genuinely distinct icon (7 unique icons for 7 types).
    expect(icons.length, types.length);
    // Every type gets a genuinely distinct background color.
    expect(backgrounds.length, types.length);

    // MUHIM must look visually distinct from ESLATMA specifically (the
    // brief's explicit example pairing).
    expect(
      iconOf(tester, const Key('explanation-block-icon-muhim')),
      isNot(iconOf(tester, const Key('explanation-block-icon-eslatma'))),
    );
    expect(
      backgroundOf(tester, const Key('explanation-block-muhim')),
      isNot(backgroundOf(tester, const Key('explanation-block-eslatma'))),
    );
  });

  testWidgets('answer_analysis items render distinct correct/incorrect icons+colors',
      (tester) async {
    const block = ExplanationBlock(
      type: 'answer_analysis',
      items: [
        AnswerAnalysisItem(position: 1, correct: false, text: 'A javobi'),
        AnswerAnalysisItem(position: 2, correct: true, text: 'B javobi'),
      ],
    );

    await tester.pumpWidget(wrap(const ExplanationView(blocks: [block])));

    final incorrectIcon = iconOf(tester, const Key('answer-analysis-item-icon-1'));
    final correctIcon = iconOf(tester, const Key('answer-analysis-item-icon-2'));
    expect(incorrectIcon, isNot(correctIcon));

    final incorrectColor = iconColorOf(tester, const Key('answer-analysis-item-icon-1'));
    final correctColor = iconColorOf(tester, const Key('answer-analysis-item-icon-2'));
    expect(incorrectColor, isNot(correctColor));
  });

  testWidgets('renders an unrecognized block type without crashing', (tester) async {
    const block = ExplanationBlock(type: 'something_new', text: 'Nomalum blok');

    await tester.pumpWidget(wrap(const ExplanationView(blocks: [block])));

    expect(find.text('Nomalum blok'), findsOneWidget);
    expect(find.byKey(const Key('explanation-block-something_new')), findsOneWidget);
  });

  testWidgets('renders legal refs when present', (tester) async {
    const blocks = [ExplanationBlock(type: 'intro', text: 'Matn')];
    const refs = [
      LegalRef(code: 'YHQ 6.13', title: 'Svetofor signallari'),
      LegalRef(code: 'YHQ 11.1', title: 'Kesishma qoidalari'),
    ];

    await tester.pumpWidget(
      wrap(const ExplanationView(blocks: blocks, legalRefs: refs)),
    );

    expect(find.byKey(const Key('explanation-legal-refs')), findsOneWidget);
    expect(find.text('YHQ 6.13'), findsOneWidget);
    expect(find.text('Svetofor signallari'), findsOneWidget);
    expect(find.text('YHQ 11.1'), findsOneWidget);
    expect(find.text('Kesishma qoidalari'), findsOneWidget);
  });

  testWidgets('omits the legal refs section when null', (tester) async {
    const blocks = [ExplanationBlock(type: 'intro', text: 'Matn')];

    await tester.pumpWidget(wrap(const ExplanationView(blocks: blocks)));

    expect(find.byKey(const Key('explanation-legal-refs')), findsNothing);
  });

  testWidgets('omits the legal refs section when empty', (tester) async {
    const blocks = [ExplanationBlock(type: 'intro', text: 'Matn')];

    await tester.pumpWidget(
      wrap(const ExplanationView(blocks: blocks, legalRefs: [])),
    );

    expect(find.byKey(const Key('explanation-legal-refs')), findsNothing);
  });
}
