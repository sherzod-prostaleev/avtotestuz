import 'package:flutter/material.dart';

/// A small chip-style widget rendering a single road sign: its image
/// thumbnail (when available) plus its name.
///
/// Used by [SignsScreen]'s catalog list (Task 6, each cell is one sign) and
/// intended for reuse by Task 7's explanation renderer (sign-code
/// references embedded inline in explanation text) — that's why it lives in
/// `shared/widgets` rather than under `features/signs`, matching the master
/// spec's §15 shared-widget list.
class SignChip extends StatelessWidget {
  const SignChip({required this.name, this.imageUrl, this.onTap, super.key});

  final String name;
  final String? imageUrl;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final imageUrl = this.imageUrl;
    final chip = Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(20),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (imageUrl != null) ...[
            ClipRRect(
              borderRadius: BorderRadius.circular(12),
              child: Image.network(
                imageUrl,
                width: 28,
                height: 28,
                fit: BoxFit.cover,
                errorBuilder: (context, error, stackTrace) =>
                    const Icon(Icons.warning_amber_outlined, size: 22),
              ),
            ),
            const SizedBox(width: 8),
          ] else ...[
            const Icon(Icons.warning_amber_outlined, size: 22),
            const SizedBox(width: 8),
          ],
          Flexible(child: Text(name, overflow: TextOverflow.ellipsis)),
        ],
      ),
    );
    if (onTap == null) return chip;
    return InkWell(
      borderRadius: BorderRadius.circular(20),
      onTap: onTap,
      child: chip,
    );
  }
}
