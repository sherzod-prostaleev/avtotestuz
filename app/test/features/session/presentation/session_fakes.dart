import 'package:avtotest_app/core/result.dart';
import 'package:avtotest_app/features/content/data/content_api.dart';
import 'package:avtotest_app/features/content/domain/category.dart';
import 'package:avtotest_app/features/content/domain/question.dart';
import 'package:avtotest_app/features/content/domain/sign.dart';
import 'package:avtotest_app/features/content/domain/variant.dart';
import 'package:avtotest_app/features/session/data/session_api.dart';
import 'package:avtotest_app/features/session/domain/session_models.dart';

/// A [SessionSummary] with `n` questions whose ids are `q1`..`qn`.
SessionSummary fakeSummary({
  required String mode,
  required List<String> ids,
  int timeLimitSec = 0,
}) {
  return SessionSummary(
    id: 's1',
    mode: mode,
    questionIds: ids,
    timeLimitSec: timeLimitSec,
    total: ids.length,
    startedAt: DateTime(2024),
  );
}

/// A question with four answers `<id>-a1`..`<id>-a4` (positions 1..4).
Question fakeQuestion(String id) {
  return Question(
    id: id,
    categoryCode: 'A',
    text: 'Savol matni $id',
    answers: [
      Answer(id: '$id-a1', position: 1, text: 'Variant A ($id)'),
      Answer(id: '$id-a2', position: 2, text: 'Variant B ($id)'),
      Answer(id: '$id-a3', position: 3, text: 'Variant C ($id)'),
      Answer(id: '$id-a4', position: 4, text: 'Variant D ($id)'),
    ],
  );
}

/// A hand-written fake [SessionApi] — implements the network boundary only, so
/// the REAL controller/screen run against it. Records every call for
/// assertions and lets a test shape `start`/`answer`/`finish` responses.
class FakeSessionApi implements SessionApi {
  FakeSessionApi({
    this.startResult,
    this.startFailure,
    this.answerFn,
    this.finishResult = const SessionResult(
      status: 'passed',
      stoppedReason: 'completed',
      score: 1,
      total: 1,
    ),
    this.variantsResult = const [],
    this.variantsFailure,
  });

  final SessionSummary? startResult;
  final Failure? startFailure;
  final AnswerResult Function(String questionId, String answerId)? answerFn;
  final SessionResult finishResult;
  final List<VariantStatus> variantsResult;
  final Failure? variantsFailure;

  final List<({String mode, String? variantId, String locale})> startCalls = [];
  final List<({String questionId, String answerId})> answerCalls = [];
  int finishCalls = 0;
  int myVariantsCalls = 0;

  @override
  Future<Result<SessionSummary>> start({
    required String mode,
    String? variantId,
    String? categoryId,
    String? signId,
    required String locale,
    int? count,
  }) async {
    startCalls.add((mode: mode, variantId: variantId, locale: locale));
    final failure = startFailure;
    if (failure != null) return Result.err(failure);
    return Result.ok(startResult!);
  }

  @override
  Future<Result<AnswerResult>> answer({
    required String sessionId,
    required String questionId,
    required String answerId,
  }) async {
    answerCalls.add((questionId: questionId, answerId: answerId));
    final fn = answerFn;
    final result = fn != null
        ? fn(questionId, answerId)
        : const AnswerResult(recorded: true, correct: true);
    return Result.ok(result);
  }

  @override
  Future<Result<SessionResult>> finish(String sessionId) async {
    finishCalls++;
    return Result.ok(finishResult);
  }

  @override
  Future<Result<SessionDetail>> get(String sessionId) =>
      throw UnimplementedError();

  @override
  Future<Result<List<VariantStatus>>> myVariants() async {
    myVariantsCalls++;
    final failure = variantsFailure;
    if (failure != null) return Result.err(failure);
    return Result.ok(variantsResult);
  }
}

/// A fake [ContentApi] that answers `question(id)` from [fakeQuestion] (or a
/// caller-supplied builder). Everything else is unused by the session screen.
class FakeContentApi implements ContentApi {
  FakeContentApi({Question Function(String id)? build})
      : _build = build ?? fakeQuestion;

  final Question Function(String id) _build;

  @override
  Future<Result<Question>> question(String id, {required String locale}) async {
    return Result.ok(_build(id));
  }

  @override
  Future<Result<List<Category>>> categories({required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<List<Variant>>> variants() => throw UnimplementedError();

  @override
  Future<Result<Variant>> variantByNumber(int n, {required String locale}) =>
      throw UnimplementedError();

  @override
  Future<Result<List<Sign>>> signs({
    String? group,
    String? query,
    required String locale,
  }) =>
      throw UnimplementedError();

  @override
  Future<Result<Sign>> signByCode(String code, {required String locale}) =>
      throw UnimplementedError();
}
