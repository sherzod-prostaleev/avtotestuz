// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'stats.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$CategoryStat {

 String get categoryCode; double get mastery; int get seen; int get correct;
/// Create a copy of CategoryStat
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$CategoryStatCopyWith<CategoryStat> get copyWith => _$CategoryStatCopyWithImpl<CategoryStat>(this as CategoryStat, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is CategoryStat&&(identical(other.categoryCode, categoryCode) || other.categoryCode == categoryCode)&&(identical(other.mastery, mastery) || other.mastery == mastery)&&(identical(other.seen, seen) || other.seen == seen)&&(identical(other.correct, correct) || other.correct == correct));
}


@override
int get hashCode => Object.hash(runtimeType,categoryCode,mastery,seen,correct);

@override
String toString() {
  return 'CategoryStat(categoryCode: $categoryCode, mastery: $mastery, seen: $seen, correct: $correct)';
}


}

/// @nodoc
abstract mixin class $CategoryStatCopyWith<$Res>  {
  factory $CategoryStatCopyWith(CategoryStat value, $Res Function(CategoryStat) _then) = _$CategoryStatCopyWithImpl;
@useResult
$Res call({
 String categoryCode, double mastery, int seen, int correct
});




}
/// @nodoc
class _$CategoryStatCopyWithImpl<$Res>
    implements $CategoryStatCopyWith<$Res> {
  _$CategoryStatCopyWithImpl(this._self, this._then);

  final CategoryStat _self;
  final $Res Function(CategoryStat) _then;

/// Create a copy of CategoryStat
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? categoryCode = null,Object? mastery = null,Object? seen = null,Object? correct = null,}) {
  return _then(_self.copyWith(
categoryCode: null == categoryCode ? _self.categoryCode : categoryCode // ignore: cast_nullable_to_non_nullable
as String,mastery: null == mastery ? _self.mastery : mastery // ignore: cast_nullable_to_non_nullable
as double,seen: null == seen ? _self.seen : seen // ignore: cast_nullable_to_non_nullable
as int,correct: null == correct ? _self.correct : correct // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [CategoryStat].
extension CategoryStatPatterns on CategoryStat {
/// A variant of `map` that fallback to returning `orElse`.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case _:
///     return orElse();
/// }
/// ```

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _CategoryStat value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _CategoryStat() when $default != null:
return $default(_that);case _:
  return orElse();

}
}
/// A `switch`-like method, using callbacks.
///
/// Callbacks receives the raw object, upcasted.
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case final Subclass2 value:
///     return ...;
/// }
/// ```

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _CategoryStat value)  $default,){
final _that = this;
switch (_that) {
case _CategoryStat():
return $default(_that);case _:
  throw StateError('Unexpected subclass');

}
}
/// A variant of `map` that fallback to returning `null`.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case _:
///     return null;
/// }
/// ```

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _CategoryStat value)?  $default,){
final _that = this;
switch (_that) {
case _CategoryStat() when $default != null:
return $default(_that);case _:
  return null;

}
}
/// A variant of `when` that fallback to an `orElse` callback.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case _:
///     return orElse();
/// }
/// ```

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String categoryCode,  double mastery,  int seen,  int correct)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _CategoryStat() when $default != null:
return $default(_that.categoryCode,_that.mastery,_that.seen,_that.correct);case _:
  return orElse();

}
}
/// A `switch`-like method, using callbacks.
///
/// As opposed to `map`, this offers destructuring.
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case Subclass2(:final field2):
///     return ...;
/// }
/// ```

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String categoryCode,  double mastery,  int seen,  int correct)  $default,) {final _that = this;
switch (_that) {
case _CategoryStat():
return $default(_that.categoryCode,_that.mastery,_that.seen,_that.correct);case _:
  throw StateError('Unexpected subclass');

}
}
/// A variant of `when` that fallback to returning `null`
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case _:
///     return null;
/// }
/// ```

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String categoryCode,  double mastery,  int seen,  int correct)?  $default,) {final _that = this;
switch (_that) {
case _CategoryStat() when $default != null:
return $default(_that.categoryCode,_that.mastery,_that.seen,_that.correct);case _:
  return null;

}
}

}

/// @nodoc


class _CategoryStat implements CategoryStat {
  const _CategoryStat({required this.categoryCode, required this.mastery, required this.seen, required this.correct});
  

@override final  String categoryCode;
@override final  double mastery;
@override final  int seen;
@override final  int correct;

/// Create a copy of CategoryStat
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$CategoryStatCopyWith<_CategoryStat> get copyWith => __$CategoryStatCopyWithImpl<_CategoryStat>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _CategoryStat&&(identical(other.categoryCode, categoryCode) || other.categoryCode == categoryCode)&&(identical(other.mastery, mastery) || other.mastery == mastery)&&(identical(other.seen, seen) || other.seen == seen)&&(identical(other.correct, correct) || other.correct == correct));
}


@override
int get hashCode => Object.hash(runtimeType,categoryCode,mastery,seen,correct);

@override
String toString() {
  return 'CategoryStat(categoryCode: $categoryCode, mastery: $mastery, seen: $seen, correct: $correct)';
}


}

/// @nodoc
abstract mixin class _$CategoryStatCopyWith<$Res> implements $CategoryStatCopyWith<$Res> {
  factory _$CategoryStatCopyWith(_CategoryStat value, $Res Function(_CategoryStat) _then) = __$CategoryStatCopyWithImpl;
@override @useResult
$Res call({
 String categoryCode, double mastery, int seen, int correct
});




}
/// @nodoc
class __$CategoryStatCopyWithImpl<$Res>
    implements _$CategoryStatCopyWith<$Res> {
  __$CategoryStatCopyWithImpl(this._self, this._then);

  final _CategoryStat _self;
  final $Res Function(_CategoryStat) _then;

/// Create a copy of CategoryStat
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? categoryCode = null,Object? mastery = null,Object? seen = null,Object? correct = null,}) {
  return _then(_CategoryStat(
categoryCode: null == categoryCode ? _self.categoryCode : categoryCode // ignore: cast_nullable_to_non_nullable
as String,mastery: null == mastery ? _self.mastery : mastery // ignore: cast_nullable_to_non_nullable
as double,seen: null == seen ? _self.seen : seen // ignore: cast_nullable_to_non_nullable
as int,correct: null == correct ? _self.correct : correct // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}

/// @nodoc
mixin _$Stats {

 List<CategoryStat> get categories; int get readinessPct; int get dueCount;
/// Create a copy of Stats
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$StatsCopyWith<Stats> get copyWith => _$StatsCopyWithImpl<Stats>(this as Stats, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Stats&&const DeepCollectionEquality().equals(other.categories, categories)&&(identical(other.readinessPct, readinessPct) || other.readinessPct == readinessPct)&&(identical(other.dueCount, dueCount) || other.dueCount == dueCount));
}


@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(categories),readinessPct,dueCount);

@override
String toString() {
  return 'Stats(categories: $categories, readinessPct: $readinessPct, dueCount: $dueCount)';
}


}

/// @nodoc
abstract mixin class $StatsCopyWith<$Res>  {
  factory $StatsCopyWith(Stats value, $Res Function(Stats) _then) = _$StatsCopyWithImpl;
@useResult
$Res call({
 List<CategoryStat> categories, int readinessPct, int dueCount
});




}
/// @nodoc
class _$StatsCopyWithImpl<$Res>
    implements $StatsCopyWith<$Res> {
  _$StatsCopyWithImpl(this._self, this._then);

  final Stats _self;
  final $Res Function(Stats) _then;

/// Create a copy of Stats
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? categories = null,Object? readinessPct = null,Object? dueCount = null,}) {
  return _then(_self.copyWith(
categories: null == categories ? _self.categories : categories // ignore: cast_nullable_to_non_nullable
as List<CategoryStat>,readinessPct: null == readinessPct ? _self.readinessPct : readinessPct // ignore: cast_nullable_to_non_nullable
as int,dueCount: null == dueCount ? _self.dueCount : dueCount // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [Stats].
extension StatsPatterns on Stats {
/// A variant of `map` that fallback to returning `orElse`.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case _:
///     return orElse();
/// }
/// ```

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Stats value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Stats() when $default != null:
return $default(_that);case _:
  return orElse();

}
}
/// A `switch`-like method, using callbacks.
///
/// Callbacks receives the raw object, upcasted.
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case final Subclass2 value:
///     return ...;
/// }
/// ```

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Stats value)  $default,){
final _that = this;
switch (_that) {
case _Stats():
return $default(_that);case _:
  throw StateError('Unexpected subclass');

}
}
/// A variant of `map` that fallback to returning `null`.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case _:
///     return null;
/// }
/// ```

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Stats value)?  $default,){
final _that = this;
switch (_that) {
case _Stats() when $default != null:
return $default(_that);case _:
  return null;

}
}
/// A variant of `when` that fallback to an `orElse` callback.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case _:
///     return orElse();
/// }
/// ```

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( List<CategoryStat> categories,  int readinessPct,  int dueCount)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Stats() when $default != null:
return $default(_that.categories,_that.readinessPct,_that.dueCount);case _:
  return orElse();

}
}
/// A `switch`-like method, using callbacks.
///
/// As opposed to `map`, this offers destructuring.
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case Subclass2(:final field2):
///     return ...;
/// }
/// ```

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( List<CategoryStat> categories,  int readinessPct,  int dueCount)  $default,) {final _that = this;
switch (_that) {
case _Stats():
return $default(_that.categories,_that.readinessPct,_that.dueCount);case _:
  throw StateError('Unexpected subclass');

}
}
/// A variant of `when` that fallback to returning `null`
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case _:
///     return null;
/// }
/// ```

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( List<CategoryStat> categories,  int readinessPct,  int dueCount)?  $default,) {final _that = this;
switch (_that) {
case _Stats() when $default != null:
return $default(_that.categories,_that.readinessPct,_that.dueCount);case _:
  return null;

}
}

}

/// @nodoc


class _Stats implements Stats {
  const _Stats({required final  List<CategoryStat> categories, required this.readinessPct, required this.dueCount}): _categories = categories;
  

 final  List<CategoryStat> _categories;
@override List<CategoryStat> get categories {
  if (_categories is EqualUnmodifiableListView) return _categories;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_categories);
}

@override final  int readinessPct;
@override final  int dueCount;

/// Create a copy of Stats
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$StatsCopyWith<_Stats> get copyWith => __$StatsCopyWithImpl<_Stats>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Stats&&const DeepCollectionEquality().equals(other._categories, _categories)&&(identical(other.readinessPct, readinessPct) || other.readinessPct == readinessPct)&&(identical(other.dueCount, dueCount) || other.dueCount == dueCount));
}


@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(_categories),readinessPct,dueCount);

@override
String toString() {
  return 'Stats(categories: $categories, readinessPct: $readinessPct, dueCount: $dueCount)';
}


}

/// @nodoc
abstract mixin class _$StatsCopyWith<$Res> implements $StatsCopyWith<$Res> {
  factory _$StatsCopyWith(_Stats value, $Res Function(_Stats) _then) = __$StatsCopyWithImpl;
@override @useResult
$Res call({
 List<CategoryStat> categories, int readinessPct, int dueCount
});




}
/// @nodoc
class __$StatsCopyWithImpl<$Res>
    implements _$StatsCopyWith<$Res> {
  __$StatsCopyWithImpl(this._self, this._then);

  final _Stats _self;
  final $Res Function(_Stats) _then;

/// Create a copy of Stats
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? categories = null,Object? readinessPct = null,Object? dueCount = null,}) {
  return _then(_Stats(
categories: null == categories ? _self._categories : categories // ignore: cast_nullable_to_non_nullable
as List<CategoryStat>,readinessPct: null == readinessPct ? _self.readinessPct : readinessPct // ignore: cast_nullable_to_non_nullable
as int,dueCount: null == dueCount ? _self.dueCount : dueCount // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}

// dart format on
