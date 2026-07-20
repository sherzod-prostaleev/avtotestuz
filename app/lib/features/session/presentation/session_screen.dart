import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../app/theme/app_theme.dart';
import '../../../core/result.dart';
import '../../../shared/widgets/answer_option.dart';
import '../../../shared/widgets/countdown_timer.dart';
import '../../../shared/widgets/question_card.dart';
import '../../../shared/widgets/question_navigator.dart';
import '../../content/domain/question.dart';
import '../../saved/presentation/saved_toggle_button.dart';
import '../domain/session_models.dart';
import '../domain/session_state.dart';
import 'session_controller.dart';

/// The route this screen navigates to when the session finishes. Task 4 wires
/// a minimal [SessionResultView] here (via `router.dart`); Task 5 supersedes
/// it with the full `SessionResultsScreen`. Kept as a shared const so the
/// screen and its tests agree on the destination.
const sessionResultRoute = '/session/result';

/// The single test-taking screen shared by all four modes
/// (`variant`/`exam`/`practice`/`mistakes`). It differs only in *chrome and
/// rules*, never in mechanics:
///
/// - **exam**: a [CountdownTimer] (fed by [SessionActive.remaining], ticked by
///   [SessionController]'s timer) + a [QuestionNavigator]; per-answer feedback
///   is WITHHELD (the server sends `correct: null`, and this screen renders no
///   correctness colours — never guessing what the server didn't send); the
///   controller auto-finishes on the server's `stopped` signal or the timer.
/// - **variant / practice / mistakes**: immediate per-answer feedback
///   (correct / incorrect / the-correct-one), no timer, no navigator grid.
///
/// The controller *owns* starting the session (keyed by a
/// [SessionStartRequest]) — mounting this screen starts the session — so there
/// is no "did the caller start a session first?" seam to get wrong.
class SessionScreen extends ConsumerStatefulWidget {
  const SessionScreen({required this.request, super.key});

  final SessionStartRequest request;

  @override
  ConsumerState<SessionScreen> createState() => _SessionScreenState();
}

class _SessionScreenState extends ConsumerState<SessionScreen> {
  /// questionId → the answerId the user tapped. Tracked here because
  /// `AnswerResult` (the server's answer response) does NOT echo back which
  /// option was chosen — so to keep the chosen option visually selected we
  /// remember it locally.
  final Map<String, String> _selected = {};

  SessionStartRequest get _request => widget.request;

  void _onSelect(String questionId, String answerId, SessionActive active) {
    // Already answered → ignore (a stray tap / F-key after answering must not
    // re-submit or move the selection).
    if (active.answered.containsKey(questionId)) return;
    setState(() => _selected[questionId] = answerId);
    ref
        .read(sessionControllerProvider(_request).notifier)
        .submitAnswer(questionId, answerId);
  }

  @override
  Widget build(BuildContext context) {
    // Navigate on finish — the core anti-seam-bug behaviour: it is not enough
    // for the controller to reach SessionUiState.finished; this screen must
    // actually route to the results view. `context.go` replaces the session
    // in the stack so "back" from results never lands on a finished session.
    ref.listen<SessionUiState>(sessionControllerProvider(_request), (
      previous,
      next,
    ) {
      if (next is SessionFinished) {
        context.go(sessionResultRoute, extra: next.result);
      }
    });

    final state = ref.watch(sessionControllerProvider(_request));

    return switch (state) {
      SessionLoading() => _scaffold(
        _request.mode,
        remaining: null,
        body: const Center(
          key: Key('session-loading'),
          child: CircularProgressIndicator(),
        ),
      ),
      SessionError(:final failure) => _scaffold(
        _request.mode,
        remaining: null,
        body: _ErrorView(failure: failure),
      ),
      // The stopped state is transient here (the controller collapses it
      // straight into finish()); show a spinner while that resolves.
      SessionStopped() || SessionFinished() => _scaffold(
        _request.mode,
        remaining: null,
        body: const Center(child: CircularProgressIndicator()),
      ),
      SessionActive() => _buildActive(state),
    };
  }

  Widget _buildActive(SessionActive active) {
    final questionId = active.summary.questionIds[active.currentIndex];
    final questionAsync = ref.watch(sessionQuestionProvider(questionId));

    return _scaffold(
      active.summary.mode,
      remaining: active.remaining,
      total: Duration(seconds: active.summary.timeLimitSec),
      body: questionAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Text('Savolni yuklab bo\'lmadi.'),
              const SizedBox(height: AppSpacing.sm),
              OutlinedButton(
                onPressed: () =>
                    ref.invalidate(sessionQuestionProvider(questionId)),
                child: const Text('Qayta urinish'),
              ),
            ],
          ),
        ),
        data: (question) => _QuestionBody(
          active: active,
          question: question,
          selectedAnswerId: _selected[questionId],
          onSelect: (answerId) => _onSelect(questionId, answerId, active),
          onJump: (index) =>
              ref.read(sessionControllerProvider(_request).notifier).jumpTo(index),
          onNext: () => ref
              .read(sessionControllerProvider(_request).notifier)
              .jumpTo(active.currentIndex + 1),
          onFinish: () =>
              ref.read(sessionControllerProvider(_request).notifier).finish(),
        ),
      ),
    );
  }

  Widget _scaffold(
    String mode, {
    required Duration? remaining,
    Duration? total,
    required Widget body,
  }) {
    return Scaffold(
      appBar: AppBar(
        title: Text(_modeTitle(mode)),
        actions: [
          if (remaining != null && total != null)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md),
              child: Center(
                child: CountdownTimer(remaining: remaining, total: total),
              ),
            ),
        ],
      ),
      body: SafeArea(child: body),
    );
  }

  static String _modeTitle(String mode) => switch (mode) {
    'exam' => 'Imtihon',
    'variant' => 'Bilet',
    'practice' => 'Mashq',
    'mistakes' => 'Xatolar ustida ishlash',
    _ => 'Test',
  };
}

/// The active-question layout: question card, answer options (with F1-F4),
/// exam-only navigator, and the next/finish control.
class _QuestionBody extends StatelessWidget {
  const _QuestionBody({
    required this.active,
    required this.question,
    required this.selectedAnswerId,
    required this.onSelect,
    required this.onJump,
    required this.onNext,
    required this.onFinish,
  });

  final SessionActive active;
  final Question question;
  final String? selectedAnswerId;
  final ValueChanged<String> onSelect;
  final ValueChanged<int> onJump;
  final VoidCallback onNext;
  final VoidCallback onFinish;

  bool get _isExam => active.summary.mode == 'exam';

  @override
  Widget build(BuildContext context) {
    final questionId = question.id;
    final result = active.answered[questionId];
    final isAnswered = result != null;
    final total = active.summary.questionIds.length;
    final isLast = active.currentIndex == total - 1;

    // Answers in A..D order (sorted by their `position`), so the label chip
    // and the F1..F4 shortcut index line up.
    final answers = [...question.answers]
      ..sort((a, b) => a.position.compareTo(b.position));

    final options = <AnswerOption>[
      for (var i = 0; i < answers.length; i++)
        AnswerOption(
          label: String.fromCharCode('A'.codeUnitAt(0) + i),
          text: answers[i].text,
          selected: selectedAnswerId == answers[i].id,
          feedback: _feedbackFor(answers[i].id, result),
          onSelect: () => onSelect(answers[i].id),
        ),
    ];

    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppSpacing.md),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            'Savol ${active.currentIndex + 1} / $total',
            key: const Key('session-progress'),
            style: Theme.of(context).textTheme.labelLarge,
          ),
          const SizedBox(height: AppSpacing.sm),
          Stack(
            children: [
              QuestionCard(question: question.text, imageUrl: question.imageUrl),
              Positioned(
                top: 4,
                right: 4,
                child: SavedToggleButton(questionId: question.id),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          AnswerOptionsGroup(options: options),
          if (_isExam) ...[
            const SizedBox(height: AppSpacing.md),
            QuestionNavigator(
              total: total,
              currentIndex: active.currentIndex,
              answeredIndices: {
                for (var i = 0; i < active.summary.questionIds.length; i++)
                  if (active.answered.containsKey(active.summary.questionIds[i]))
                    i,
              },
              onJump: onJump,
            ),
          ],
          const SizedBox(height: AppSpacing.md),
          FilledButton(
            key: const Key('session-next-button'),
            // Must answer the current question before advancing/finishing.
            onPressed: isAnswered ? (isLast ? onFinish : onNext) : null,
            child: Text(isLast ? 'Yakunlash' : 'Keyingi'),
          ),
        ],
      ),
    );
  }

  /// The feedback treatment for [answerId], or null (no colour) when feedback
  /// must be withheld.
  ///
  /// Anti-cheat: feedback is shown ONLY when the server actually sent
  /// correctness for this answer. In exam mode `result.correct` is null, so
  /// this returns null for every option — the UI never infers correctness the
  /// server withheld. (It also returns null before the question is answered.)
  AnswerFeedback? _feedbackFor(String answerId, AnswerResult? result) {
    if (result == null) return null;
    if (result.correct == null) return null; // withheld (exam) — never guess.

    final wasChosen = selectedAnswerId == answerId;
    if (wasChosen) {
      return result.correct! ? AnswerFeedback.correct : AnswerFeedback.incorrect;
    }
    if (result.correctAnswerId != null && result.correctAnswerId == answerId) {
      return AnswerFeedback.theCorrectOne;
    }
    return null;
  }
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.failure});

  final Failure failure;

  @override
  Widget build(BuildContext context) {
    // Distinct, non-alarming copy for the two "this is expected, not a crash"
    // codes; the raw server message otherwise.
    final message = switch (failure.code) {
      'vip_required' =>
        'Bu bo\'lim uchun faol obuna kerak. Obuna hozircha bu versiyada '
            'mavjud emas.',
      'daily_limit_reached' =>
        'Bugungi bepul limitga yetdingiz. Ertaga yana davom etishingiz mumkin.',
      _ => failure.message,
    };

    return Center(
      key: const Key('session-error-view'),
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.lg),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(message, textAlign: TextAlign.center),
            const SizedBox(height: AppSpacing.md),
            FilledButton(
              onPressed: () => context.go('/'),
              child: const Text('Bosh sahifa'),
            ),
          ],
        ),
      ),
    );
  }
}

/// Minimal results view (Task 4). Task 5's `SessionResultsScreen` supersedes
/// this with a richer breakdown; kept here so Task 4's finish→navigate seam
/// has a real route to land on and be tested against.
class SessionResultView extends StatelessWidget {
  const SessionResultView({required this.result, super.key});

  final SessionResult result;

  @override
  Widget build(BuildContext context) {
    final statusLabel = switch (result.status) {
      'passed' => 'O\'tdingiz',
      'failed' => 'O\'ta olmadingiz',
      'abandoned' => 'Sessiya to\'xtatildi',
      _ => result.status,
    };
    final reasonLabel = switch (result.stoppedReason) {
      'completed' => 'Barcha savollar yakunlandi',
      'time_up' => 'Vaqt tugadi',
      'too_many_errors' => 'Xatolar soni ko\'payib ketdi',
      _ => result.stoppedReason,
    };

    return Scaffold(
      key: const Key('session-result-view'),
      appBar: AppBar(title: const Text('Natija')),
      body: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  statusLabel,
                  style: Theme.of(context).textTheme.headlineSmall,
                ),
                const SizedBox(height: AppSpacing.md),
                Text(
                  '${result.score} / ${result.total}',
                  key: const Key('session-result-score'),
                  style: Theme.of(context).textTheme.displaySmall,
                ),
                const SizedBox(height: AppSpacing.sm),
                Text(reasonLabel, textAlign: TextAlign.center),
                const SizedBox(height: AppSpacing.lg),
                FilledButton(
                  onPressed: () => context.go('/'),
                  child: const Text('Bosh sahifa'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
