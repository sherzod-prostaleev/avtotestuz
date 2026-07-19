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
/// switcher, a theme toggle, logout, and honest "coming soon" placeholders
/// for the richer screens Plan 07 builds (variants/practice/mistakes/
/// stats) — no half-built fake versions of those screens here.
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
      padding: const EdgeInsets.all(AppSpacing.md),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            profile.name,
            key: const Key('profileNameText'),
            style: Theme.of(context).textTheme.headlineSmall,
          ),
          const SizedBox(height: AppSpacing.sm),
          Text(profile.phone, key: const Key('profilePhoneText')),
          const SizedBox(height: AppSpacing.sm),
          Text(
            entitlement.active ? l10n.vipActiveLabel : l10n.vipInactiveLabel,
            key: const Key('vipStatusText'),
          ),
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
          GridView.count(
            crossAxisCount: 2,
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            mainAxisSpacing: AppSpacing.sm,
            crossAxisSpacing: AppSpacing.sm,
            children: [
              AppCard(
                key: const Key('navVariants'),
                onTap: () => context.push('/variants'),
                child: Center(
                  child: Text(l10n.navVariantsLabel, textAlign: TextAlign.center),
                ),
              ),
              AppCard(
                key: const Key('navPractice'),
                onTap: () => context.push('/practice'),
                child: Center(
                  child: Text(l10n.navPracticeLabel, textAlign: TextAlign.center),
                ),
              ),
              AppCard(
                key: const Key('navMistakes'),
                onTap: () => context.push('/mistakes'),
                child: Center(
                  child: Text(l10n.navMistakesLabel, textAlign: TextAlign.center),
                ),
              ),
              _ComingSoonNav(
                key: const Key('navStats'),
                title: l10n.navStatsLabel,
                subtitle: l10n.comingSoon,
              ),
            ],
          ),
        ],
      ),
    );
  }
}

/// An honest "coming soon" placeholder nav entry — disabled-looking, no
/// `onTap`, and no attempt to imitate the real variants/practice/mistakes/
/// stats screens Plan 07 builds.
class _ComingSoonNav extends StatelessWidget {
  const _ComingSoonNav({super.key, required this.title, required this.subtitle});

  final String title;
  final String subtitle;

  @override
  Widget build(BuildContext context) {
    return AppCard(
      enabled: false,
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Text(title, textAlign: TextAlign.center),
          const SizedBox(height: AppSpacing.sm),
          Text(
            subtitle,
            textAlign: TextAlign.center,
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
      ),
    );
  }
}
