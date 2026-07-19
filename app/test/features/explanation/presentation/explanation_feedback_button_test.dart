import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/explanation/data/explanation_api.dart';
import 'package:avtotest_app/features/explanation/presentation/explanation_feedback_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockExplanationApi extends Mock implements ExplanationApi {}

void main() {
  late MockExplanationApi api;

  setUp(() {
    api = MockExplanationApi();
    registerFallbackValue(false);
  });

  Widget wrap(String questionId) {
    return ProviderScope(
      overrides: [explanationApiProvider.overrideWithValue(api)],
      child: MaterialApp(
        home: Scaffold(
          body: ExplanationFeedbackButton(questionId: questionId),
        ),
      ),
    );
  }

  testWidgets('tapping thumbs up calls feedback(helpful: true) with the right questionId',
      (tester) async {
    when(() => api.feedback(questionId: any(named: 'questionId'), helpful: any(named: 'helpful')))
        .thenAnswer((_) async => const Result<void>.ok(null));

    await tester.pumpWidget(wrap('q-42'));
    await tester.tap(find.byKey(const Key('explanation-feedback-thumbs-up')));
    await tester.pump();

    verify(() => api.feedback(questionId: 'q-42', helpful: true)).called(1);
    expect(find.byKey(const Key('explanation-feedback-confirmation')), findsOneWidget);
  });

  testWidgets('tapping thumbs down calls feedback(helpful: false) with the right questionId',
      (tester) async {
    when(() => api.feedback(questionId: any(named: 'questionId'), helpful: any(named: 'helpful')))
        .thenAnswer((_) async => const Result<void>.ok(null));

    await tester.pumpWidget(wrap('q-99'));
    await tester.tap(find.byKey(const Key('explanation-feedback-thumbs-down')));
    await tester.pump();

    verify(() => api.feedback(questionId: 'q-99', helpful: false)).called(1);
    expect(find.byKey(const Key('explanation-feedback-confirmation')), findsOneWidget);
  });

  testWidgets('a not_found response shows an inline message without throwing',
      (tester) async {
    when(() => api.feedback(questionId: any(named: 'questionId'), helpful: any(named: 'helpful')))
        .thenAnswer(
      (_) async => const Result<void>.err(
        Failure(code: 'not_found', message: 'explanation not found'),
      ),
    );

    await tester.pumpWidget(wrap('q-missing'));

    // No exception should escape either the tap or the following pump.
    await tester.tap(find.byKey(const Key('explanation-feedback-thumbs-up')));
    await tester.pump();

    expect(tester.takeException(), isNull);
    expect(find.byKey(const Key('explanation-feedback-not-found')), findsOneWidget);
    expect(find.text('Hali izoh mavjud emas.'), findsOneWidget);
  });

  testWidgets('a generic error response shows an inline error message without throwing',
      (tester) async {
    when(() => api.feedback(questionId: any(named: 'questionId'), helpful: any(named: 'helpful')))
        .thenAnswer(
      (_) async => const Result<void>.err(
        Failure(code: 'network_error', message: 'boom'),
      ),
    );

    await tester.pumpWidget(wrap('q-1'));
    await tester.tap(find.byKey(const Key('explanation-feedback-thumbs-down')));
    await tester.pump();

    expect(tester.takeException(), isNull);
    expect(find.byKey(const Key('explanation-feedback-error')), findsOneWidget);
  });
}
