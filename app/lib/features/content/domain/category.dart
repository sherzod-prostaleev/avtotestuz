import 'package:freezed_annotation/freezed_annotation.dart';

part 'category.freezed.dart';

/// A learning category (e.g. "belgilar", "harakat qoidalari"), matching
/// `GET /categories`' `data[]` items exactly as returned by
/// `backend/internal/content/dto.go`'s `CategoryDTO`
/// (`backend/internal/content/handlers.go`'s `listCategories`, confirmed by
/// reading both directly, not just README prose):
/// ```
/// { "code": string, "name": string, "sort_order": int }
/// ```
/// `name` is locale-resolved server-side from the `locale=` query param
/// (falling back to `uz-Latn`, surfaced via the envelope's `meta.fallback` —
/// not modeled here, Task 1 only needs `data`).
@freezed
abstract class Category with _$Category {
  const factory Category({
    required String code,
    required String name,
    required int sortOrder,
  }) = _Category;
}
