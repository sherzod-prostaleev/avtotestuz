import 'package:avtotest_app/shared/widgets/question_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders the question text', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: QuestionCard(question: 'Svetofor sariq chiroq yonganda?', imageUrl: null),
      ),
    );

    expect(find.text('Svetofor sariq chiroq yonganda?'), findsOneWidget);
  });

  testWidgets('renders an image when imageUrl is provided', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: QuestionCard(
          question: 'Belgini tanlang',
          imageUrl: 'https://example.test/sign.png',
        ),
      ),
    );
    // Let the (failing, since there's no real network in tests) image
    // request settle so its errorBuilder can run without leaving a pending
    // timer/frame around.
    await tester.pump(const Duration(milliseconds: 50));

    expect(find.byType(Image), findsOneWidget);
    final image = tester.widget<Image>(find.byType(Image));
    final provider = image.image as NetworkImage;
    expect(provider.url, 'https://example.test/sign.png');
  });

  testWidgets('omits the image entirely when imageUrl is null', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: QuestionCard(question: 'Matnli savol', imageUrl: null),
      ),
    );

    expect(find.byType(Image), findsNothing);
  });
}
