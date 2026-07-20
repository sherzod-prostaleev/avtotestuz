import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../app/l10n/app_localizations.dart';
import '../../../app/theme/app_theme.dart';
import '../../../core/result.dart';
import '../../../shared/widgets/answer_option.dart';
import '../../../shared/widgets/countdown_timer.dart';
import '../../../shared/widgets/question_card.dart';
import '../../../shared/widgets/question_navigator.dart';
import '../../billing/presentation/vip_required_screen.dart';
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
      } else if (next is SessionError &&
          next.failure.code == 'vip_required') {
        // `vip_required` (402) is a KIND of block, not a transient error: it
        // gets a dedicated paywall-style screen (per the plan's Global
        // Constraints) rather than inline error copy. Navigate to the shared
        // [VipRequiredScreen] so every gated entry path (bilet #2+, exam,
        // mistakes) lands on one consistent upsell surface. `daily_limit_reached`
        // stays inline (see the build switch below) — that one is just a
        // message, a different kind of thing.
        context.go(vipRequiredRoute);
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
      // `vip_required` navigates away to [VipRequiredScreen] (handled in the
      // `ref.listen` above) — render a spinner for the single frame before
      // that navigation lands, never the inline error copy, so the gate is a
      // dedicated screen and not a flash of inline text. Every other failure
      // (including `daily_limit_reached`, which stays a message) renders the
      // inline [_ErrorView].
      SessionError(:final failure) => failure.code == 'vip_required'
          ? _scaffold(
              _request.mode,
              remaining: null,
              body: const Center(child: CircularProgressIndicator()),
            )
          : _scaffold(
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
    final l10n = AppLocalizations.of(context)!;
    final questionId = active.summary.questionIds[active.currentIndex];
    final questionAsync = ref.watch(sessionQuestionProvider(questionId));
    final total = active.summary.questionIds.length;
    final isAnswered = active.answered.containsKey(questionId);
    final isLast = active.currentIndex == total - 1;

    return _scaffold(
      active.summary.mode,
      remaining: active.remaining,
      total: Duration(seconds: active.summary.timeLimitSec ?? 0),
      // Bookmark for the CURRENT question lives up in the header chrome — the
      // same top-bar slot the reference screenshots put it in — instead of
      // floating over the question card.
      headerTrailing: SavedToggleButton(questionId: questionId),
      // The primary advance/finish control is a persistent bottom action bar
      // (always in reach, never scrolled past), not a button buried at the end
      // of the scrolling content.
      bottomBar: _SessionBottomBar(
        isAnswered: isAnswered,
        isLast: isLast,
        onNext: () => ref
            .read(sessionControllerProvider(_request).notifier)
            .jumpTo(active.currentIndex + 1),
        onFinish: () => ref
            .read(sessionControllerProvider(_request).notifier)
            .finish(),
      ),
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (active.pendingRetryQuestionId != null)
            _AnswerRetryBanner(
              onRetry: () => ref
                  .read(sessionControllerProvider(_request).notifier)
                  .retryPendingAnswer(),
            ),
          Expanded(
            child: questionAsync.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (error, _) => Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(l10n.sessionQuestionLoadError),
                    const SizedBox(height: AppSpacing.sm),
                    OutlinedButton(
                      onPressed: () =>
                          ref.invalidate(sessionQuestionProvider(questionId)),
                      child: Text(l10n.retryButton),
                    ),
                  ],
                ),
              ),
              data: (question) => _QuestionBody(
                active: active,
                question: question,
                selectedAnswerId: _selected[questionId],
                onSelect: (answerId) => _onSelect(questionId, answerId, active),
                onJump: (index) => ref
                    .read(sessionControllerProvider(_request).notifier)
                    .jumpTo(index),
              ),
            ),
          ),
        ],
      ),
    );
  }

  /// The screen frame shared by every state (loading / error / active): a
  /// custom themed header (mode title, an optional current-question bookmark,
  /// and — exam only — the prominent centered countdown), the state's [body],
  /// and an optional persistent [bottomBar]. Deliberately a bespoke header
  /// rather than a plain [AppBar] so the top chrome matches the reference
  /// screenshots' rounded, chip-led treatment.
  Widget _scaffold(
    String mode, {
    required Duration? remaining,
    Duration? total,
    required Widget body,
    Widget? headerTrailing,
    Widget? bottomBar,
  }) {
    final l10n = AppLocalizations.of(context)!;
    final showTimer = remaining != null && total != null;
    return Scaffold(
      body: SafeArea(
        child: Column(
          children: [
            _SessionHeader(
              title: _modeTitle(l10n, mode),
              trailing: headerTrailing,
              timer: showTimer
                  ? CountdownTimer(remaining: remaining, total: total)
                  : null,
            ),
            Expanded(child: body),
            ?bottomBar,
          ],
        ),
      ),
    );
  }

  static String _modeTitle(AppLocalizations l10n, String mode) => switch (mode) {
    'exam' => l10n.sessionTitleExam,
    'variant' => l10n.sessionTitleVariant,
    'practice' => l10n.sessionTitlePractice,
    'mistakes' => l10n.sessionTitleMistakes,
    _ => l10n.sessionTitleDefault,
  };
}

/// Inline banner shown when [SessionActive.pendingRetryQuestionId] is set —
/// i.e. the most recent answer submission failed on a transient API error.
/// Keeps the session on-screen (never a full-screen error) and offers a
/// single tap to resubmit the exact same questionId/answerId pair.
class _AnswerRetryBanner extends StatelessWidget {
  const _AnswerRetryBanner({required this.onRetry});

  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;
    return Material(
      key: const Key('answer-retry-banner'),
      color: scheme.errorContainer,
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.md,
          vertical: AppSpacing.sm,
        ),
        child: Row(
          children: [
            Expanded(
              child: Text(
                l10n.sessionAnswerRetryMessage,
                style: TextStyle(color: scheme.onErrorContainer),
              ),
            ),
            TextButton(
              key: const Key('answer-retry-button'),
              onPressed: onRetry,
              child: Text(l10n.retryButton),
            ),
          ],
        ),
      ),
    );
  }
}

/// The screen's top chrome: the mode title on the left, an optional [trailing]
/// action (the current-question bookmark) on the right, and — when a [timer] is
/// supplied (exam mode) — the countdown given its own prominent, centered row
/// beneath the title. A bespoke header (not an [AppBar]) so it can carry this
/// two-tier layout and the reference screenshots' rounded chip treatment.
class _SessionHeader extends StatelessWidget {
  const _SessionHeader({required this.title, this.trailing, this.timer});

  final String title;
  final Widget? trailing;
  final Widget? timer;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.md,
        AppSpacing.sm,
        AppSpacing.md,
        AppSpacing.md,
      ),
      decoration: BoxDecoration(
        color: theme.colorScheme.surface,
        border: Border(
          bottom: BorderSide(color: theme.colorScheme.outlineVariant),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  title,
                  style: theme.textTheme.titleLarge,
                ),
              ),
              ?trailing,
            ],
          ),
          if (timer != null) ...[
            const SizedBox(height: AppSpacing.md),
            timer!,
          ],
        ],
      ),
    );
  }
}

/// The persistent bottom action bar: a single full-width advance/finish pill,
/// pinned below the scrolling question so it's always in reach. Disabled until
/// the current question is answered (mirrors the exam/variant "answer before
/// you move on" rule). Shows "finish" copy on the last question, "next"
/// otherwise.
class _SessionBottomBar extends StatelessWidget {
  const _SessionBottomBar({
    required this.isAnswered,
    required this.isLast,
    required this.onNext,
    required this.onFinish,
  });

  final bool isAnswered;
  final bool isLast;
  final VoidCallback onNext;
  final VoidCallback onFinish;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.md,
        AppSpacing.sm,
        AppSpacing.md,
        AppSpacing.md,
      ),
      decoration: BoxDecoration(
        color: theme.colorScheme.surface,
        border: Border(
          top: BorderSide(color: theme.colorScheme.outlineVariant),
        ),
      ),
      child: SizedBox(
        width: double.infinity,
        child: FilledButton(
          key: const Key('session-next-button'),
          // Must answer the current question before advancing/finishing.
          onPressed: isAnswered ? (isLast ? onFinish : onNext) : null,
          child: Text(
            isLast ? l10n.sessionFinishButton : l10n.sessionNextButton,
          ),
        ),
      ),
    );
  }
}

/// The active-question layout: a small progress label, the question card, the
/// answer options (with F1-F4) and — exam only — the question navigator. The
/// advance/finish control is NOT here; it lives in the persistent
/// [_SessionBottomBar] the screen frame keeps pinned below this scroll view.
class _QuestionBody extends StatelessWidget {
  const _QuestionBody({
    required this.active,
    required this.question,
    required this.selectedAnswerId,
    required this.onSelect,
    required this.onJump,
  });

  final SessionActive active;
  final Question question;
  final String? selectedAnswerId;
  final ValueChanged<String> onSelect;
  final ValueChanged<int> onJump;

  bool get _isExam => active.summary.mode == 'exam';

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final questionId = question.id;
    final result = active.answered[questionId];
    final total = active.summary.questionIds.length;

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
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.md,
        AppSpacing.md,
        AppSpacing.md,
        AppSpacing.lg,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            l10n.sessionProgressLabel(active.currentIndex + 1, total),
            key: const Key('session-progress'),
            style: theme.textTheme.labelMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              letterSpacing: 0.5,
            ),
          ),
          const SizedBox(height: AppSpacing.md),
          QuestionCard(question: question.text, imageUrl: question.imageUrl),
          const SizedBox(height: AppSpacing.lg),
          AnswerOptionsGroup(options: options),
          if (_isExam) ...[
            const SizedBox(height: AppSpacing.lg),
            Text(
              l10n.sessionNavigatorLabel,
              style: theme.textTheme.titleSmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: AppSpacing.sm),
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
    final l10n = AppLocalizations.of(context)!;
    // `vip_required` never reaches here — it navigates to [VipRequiredScreen]
    // (a dedicated paywall-style screen) from the build switch above. This
    // view handles the "expected, stay-on-screen message" codes: a
    // non-alarming line for `daily_limit_reached`, the raw server message
    // otherwise.
    final message = switch (failure.code) {
      'daily_limit_reached' => l10n.sessionDailyLimitError,
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
              child: Text(l10n.homeButton),
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
    final l10n = AppLocalizations.of(context)!;
    final statusLabel = switch (result.status) {
      'passed' => l10n.sessionStatusPassedLabel,
      'failed' => l10n.sessionStatusFailedLabel,
      'abandoned' => l10n.sessionResultViewAbandonedLabel,
      _ => result.status,
    };
    final reasonLabel = switch (result.stoppedReason) {
      'completed' => l10n.sessionReasonCompletedLabel,
      'time_up' => l10n.sessionReasonTimeUpLabel,
      'too_many_errors' => l10n.sessionReasonTooManyErrorsLabel,
      _ => result.stoppedReason,
    };

    return Scaffold(
      key: const Key('session-result-view'),
      appBar: AppBar(title: Text(l10n.sessionResultTitle)),
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
                  child: Text(l10n.homeButton),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
