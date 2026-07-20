import 'package:avtotest_app/shared/widgets/sign_chip.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _wrap(Widget child) => MaterialApp(home: Scaffold(body: Center(child: child)));

void main() {
  testWidgets('renders the sign name', (tester) async {
    await tester.pumpWidget(_wrap(const SignChip(name: 'Bosh yo\'l')));

    expect(find.text('Bosh yo\'l'), findsOneWidget);
  });

  testWidgets('with no imageUrl, renders the warning-icon fallback', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap(const SignChip(name: 'Bosh yo\'l')));

    expect(find.byIcon(Icons.warning_amber_outlined), findsOneWidget);
    expect(find.byType(Image), findsNothing);
  });

  testWidgets('with an imageUrl, attempts to render a network image', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        const SignChip(
          name: 'Bosh yo\'l',
          imageUrl: 'https://example.test/sign.png',
        ),
      ),
    );

    // The image itself is present in the tree (its load may fail in this
    // offline test environment and fall back to the errorBuilder icon after
    // the async load rejects — either way, the presence of an Image widget
    // for a non-null imageUrl, rather than the eager icon-only fallback
    // branch, is what this test discriminates).
    expect(find.byType(Image), findsOneWidget);
  });

  testWidgets('with no onTap, is not wrapped in an InkWell', (tester) async {
    await tester.pumpWidget(_wrap(const SignChip(name: 'Bosh yo\'l')));

    expect(find.byType(InkWell), findsNothing);
  });

  testWidgets('with onTap, wraps in an InkWell and calls it when tapped', (
    tester,
  ) async {
    var tapped = false;
    await tester.pumpWidget(
      _wrap(SignChip(name: 'Bosh yo\'l', onTap: () => tapped = true)),
    );

    expect(find.byType(InkWell), findsOneWidget);

    await tester.tap(find.byType(InkWell));
    await tester.pump();

    expect(tapped, isTrue);
  });
}
