import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../app/l10n/app_localizations.dart';
import '../../../app/locale/locale_provider.dart';
import '../../../app/theme/app_theme.dart';
import '../../../app/theme/theme_mode_provider.dart';
import '../../../shared/widgets/app_card.dart';
import '../../../shared/widgets/empty_state.dart';
import '../../auth/presentation/auth_controller.dart';
import '../../profile/domain/profile.dart';
import '../../profile/presentation/profile_controller.dart';

/// Authenticated landing screen, reachable at `/` once the router's auth
/// guard (`app/lib/app/router.dart`) confirms `AuthAuthenticated`. Shows the
/// current profile/VIP status (from [profileControllerProvider]), a locale
/// switcher, a theme toggle, logout, a saved-questions shortcut, and the
/// nav grid to Plan 07's screens (variants/practice/mistakes/stats).
class HomeShell extends ConsumerWidget {
  const HomeShell({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profileState = ref.watch(profileControllerProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.appTitle),
        actions: [
          IconButton(
            key: const Key('savedButton'),
            icon: const Icon(Icons.bookmark),
            tooltip: 'Saqlangan savollar',
            onPressed: () => context.push('/saved'),
          ),
          const _ThemeToggleButton(),
          IconButton(
            key: const Key('logoutButton'),
            icon: const Icon(Icons.logout),
            tooltip: l10n.logout,
            onPressed: () =>
                ref.read(authControllerProvider.notifier).logout(),
          ),
        ],
      ),
      body: profileState.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, stackTrace) => EmptyState(
          key: const Key('profileErrorState'),
          message: error is ProfileFetchFailure
              ? error.failure.message
              : l10n.profileLoadError,
          onRetry: () => ref.invalidate(profileControllerProvider),
          retryLabel: l10n.retryButton,
        ),
        data: (value) =>
            _HomeBody(profile: value.profile, entitlement: value.entitlement),
      ),
    );
  }
}

/// Toggles between [ThemeMode.light]/[ThemeMode.dark]. `ThemeMode.system`
/// is a valid persisted value (`themeModeProvider` supports it) but this
/// button only ever toggles between the two explicit modes — matching the
/// "keep it simple, a two-way toggle is enough" scope of this foundation
/// screen; a three-way system/light/dark picker is not required here.
class _ThemeToggleButton extends ConsumerWidget {
  const _ThemeToggleButton();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final mode = ref.watch(themeModeProvider);
    final isDark = mode == ThemeMode.dark;
    return IconButton(
      key: const Key('themeToggleButton'),
      icon: Icon(isDark ? Icons.light_mode : Icons.dark_mode),
      tooltip: l10n.themeToggleTooltip,
      onPressed: () => ref
          .read(themeModeProvider.notifier)
          .setThemeMode(isDark ? ThemeMode.light : ThemeMode.dark),
    );
  }
}

class _HomeBody extends ConsumerWidget {
  const _HomeBody({required this.profile, required this.entitlement});

  final Profile profile;
  final Entitlement entitlement;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final locale = ref.watch(localeProvider);
    final isUzLatn = locale.languageCode == 'uz' && locale.scriptCode != 'Cyrl';
    final isUzCyrl = locale.languageCode == 'uz' && locale.scriptCode == 'Cyrl';
    final isRu = locale.languageCode == 'ru';

    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.md,
        AppSpacing.sm,
        AppSpacing.md,
        AppSpacing.xl,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _ProfileHeader(profile: profile, entitlement: entitlement),
          const SizedBox(height: AppSpacing.lg),
          Wrap(
            spacing: AppSpacing.sm,
            children: [
              ChoiceChip(
                key: const Key('localeUzLatn'),
                label: const Text("O'zbekcha"),
                selected: isUzLatn,
                onSelected: (_) => ref
                    .read(localeProvider.notifier)
                    .setLocale(const Locale('uz')),
              ),
              ChoiceChip(
                key: const Key('localeUzCyrl'),
                label: const Text('Ўзбекча'),
                selected: isUzCyrl,
                onSelected: (_) => ref.read(localeProvider.notifier).setLocale(
                      const Locale.fromSubtags(
                        languageCode: 'uz',
                        scriptCode: 'Cyrl',
                      ),
                    ),
              ),
              ChoiceChip(
                key: const Key('localeRu'),
                label: const Text('Русский'),
                selected: isRu,
                onSelected: (_) => ref
                    .read(localeProvider.notifier)
                    .setLocale(const Locale('ru')),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.lg),
          GridView(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 2,
              mainAxisSpacing: AppSpacing.md,
              crossAxisSpacing: AppSpacing.md,
              mainAxisExtent: 152,
            ),
            children: [
              _NavCard(
                navKey: const Key('navVariants'),
                icon: Icons.quiz_rounded,
                title: l10n.navVariantsLabel,
                subtitle: l10n.navVariantsSubtitle,
                onTap: () => context.push('/variants'),
              ),
              _NavCard(
                navKey: const Key('navPractice'),
                icon: Icons.fitness_center_rounded,
                title: l10n.navPracticeLabel,
                subtitle: l10n.navPracticeSubtitle,
                onTap: () => context.push('/practice'),
              ),
              _NavCard(
                navKey: const Key('navMistakes'),
                icon: Icons.history_edu_rounded,
                title: l10n.navMistakesLabel,
                subtitle: l10n.navMistakesSubtitle,
                onTap: () => context.push('/mistakes'),
              ),
              _NavCard(
                navKey: const Key('navStats'),
                icon: Icons.bar_chart_rounded,
                title: l10n.navStatsLabel,
                subtitle: l10n.navStatsSubtitle,
                onTap: () => context.push('/stats'),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

/// Hero header for the home shell: greeting + avatar initial + name/phone and
/// a VIP status badge. Keeps the existing `profileNameText`/`profilePhoneText`/
/// `vipStatusText` keys (and their exact text) that the test suite relies on.
class _ProfileHeader extends StatelessWidget {
  const _ProfileHeader({required this.profile, required this.entitlement});

  final Profile profile;
  final Entitlement entitlement;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final trimmed = profile.name.trim();
    final initial = trimmed.isEmpty ? '?' : trimmed.substring(0, 1).toUpperCase();

    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 56,
                height: 56,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: scheme.primaryContainer,
                  shape: BoxShape.circle,
                ),
                child: Text(
                  initial,
                  style: theme.textTheme.headlineSmall?.copyWith(
                    color: scheme.onPrimaryContainer,
                  ),
                ),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      l10n.homeGreetingLabel,
                      style: theme.textTheme.labelMedium?.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      profile.name,
                      key: const Key('profileNameText'),
                      style: theme.textTheme.headlineSmall,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 2),
                    Text(
                      profile.phone,
                      key: const Key('profilePhoneText'),
                      style: theme.textTheme.bodySmall,
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          _VipBadge(
            active: entitlement.active,
            label: entitlement.active
                ? l10n.vipActiveLabel
                : l10n.vipInactiveLabel,
          ),
        ],
      ),
    );
  }
}

/// Small pill rendering the VIP/subscription status. Active reads as a brand
/// "premium" moment; inactive is a quiet outlined chip. The `vipStatusText`
/// key + exact label string are preserved for the test suite.
class _VipBadge extends StatelessWidget {
  const _VipBadge({required this.active, required this.label});

  final bool active;
  final String label;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    return Align(
      alignment: Alignment.centerLeft,
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.md,
          vertical: AppSpacing.sm,
        ),
        decoration: BoxDecoration(
          color: active ? scheme.primaryContainer : Colors.transparent,
          borderRadius: BorderRadius.circular(AppRadius.chip),
          border: Border.all(
            color: active
                ? scheme.primary.withValues(alpha: 0.45)
                : scheme.outlineVariant,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              active
                  ? Icons.workspace_premium_rounded
                  : Icons.lock_outline_rounded,
              size: 18,
              color: active ? scheme.primary : scheme.onSurfaceVariant,
            ),
            const SizedBox(width: AppSpacing.xs),
            Text(
              label,
              key: const Key('vipStatusText'),
              style: theme.textTheme.labelLarge?.copyWith(
                color: active ? scheme.onSurface : scheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// A single nav-grid entry: icon tile + title + subtitle in an [AppCard]. The
/// `navKey` is forwarded to the [AppCard] so the existing nav keys keep
/// resolving; the label text is unchanged, only enriched with an icon and a
/// one-line subtitle for hierarchy.
class _NavCard extends StatelessWidget {
  const _NavCard({
    required this.navKey,
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.onTap,
  });

  final Key navKey;
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    return AppCard(
      key: navKey,
      onTap: onTap,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Container(
            width: 44,
            height: 44,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: scheme.primaryContainer,
              borderRadius: BorderRadius.circular(AppRadius.chip),
            ),
            child: Icon(icon, color: scheme.onPrimaryContainer),
          ),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: theme.textTheme.titleMedium,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              const SizedBox(height: 2),
              Text(
                subtitle,
                style: theme.textTheme.bodySmall,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
            ],
          ),
        ],
      ),
    );
  }
}
