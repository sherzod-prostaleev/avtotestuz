// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'saved_entry.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$SavedEntry {

 String get questionId; DateTime get createdAt;
/// Create a copy of SavedEntry
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SavedEntryCopyWith<SavedEntry> get copyWith => _$SavedEntryCopyWithImpl<SavedEntry>(this as SavedEntry, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SavedEntry&&(identical(other.questionId, questionId) || other.questionId == questionId)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt));
}


@override
int get hashCode => Object.hash(runtimeType,questionId,createdAt);

@override
String toString() {
  return 'SavedEntry(questionId: $questionId, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class $SavedEntryCopyWith<$Res>  {
  factory $SavedEntryCopyWith(SavedEntry value, $Res Function(SavedEntry) _then) = _$SavedEntryCopyWithImpl;
@useResult
$Res call({
 String questionId, DateTime createdAt
});




}
/// @nodoc
class _$SavedEntryCopyWithImpl<$Res>
    implements $SavedEntryCopyWith<$Res> {
  _$SavedEntryCopyWithImpl(this._self, this._then);

  final SavedEntry _self;
  final $Res Function(SavedEntry) _then;

/// Create a copy of SavedEntry
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? questionId = null,Object? createdAt = null,}) {
  return _then(_self.copyWith(
questionId: null == questionId ? _self.questionId : questionId // ignore: cast_nullable_to_non_nullable
as String,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime,
  ));
}

}


/// Adds pattern-matching-related methods to [SavedEntry].
extension SavedEntryPatterns on SavedEntry {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SavedEntry value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SavedEntry() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SavedEntry value)  $default,){
final _that = this;
switch (_that) {
case _SavedEntry():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SavedEntry value)?  $default,){
final _that = this;
switch (_that) {
case _SavedEntry() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String questionId,  DateTime createdAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SavedEntry() when $default != null:
return $default(_that.questionId,_that.createdAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String questionId,  DateTime createdAt)  $default,) {final _that = this;
switch (_that) {
case _SavedEntry():
return $default(_that.questionId,_that.createdAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String questionId,  DateTime createdAt)?  $default,) {final _that = this;
switch (_that) {
case _SavedEntry() when $default != null:
return $default(_that.questionId,_that.createdAt);case _:
  return null;

}
}

}

/// @nodoc


class _SavedEntry implements SavedEntry {
  const _SavedEntry({required this.questionId, required this.createdAt});
  

@override final  String questionId;
@override final  DateTime createdAt;

/// Create a copy of SavedEntry
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SavedEntryCopyWith<_SavedEntry> get copyWith => __$SavedEntryCopyWithImpl<_SavedEntry>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SavedEntry&&(identical(other.questionId, questionId) || other.questionId == questionId)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt));
}


@override
int get hashCode => Object.hash(runtimeType,questionId,createdAt);

@override
String toString() {
  return 'SavedEntry(questionId: $questionId, createdAt: $createdAt)';
}


}

/// @nodoc
abstract mixin class _$SavedEntryCopyWith<$Res> implements $SavedEntryCopyWith<$Res> {
  factory _$SavedEntryCopyWith(_SavedEntry value, $Res Function(_SavedEntry) _then) = __$SavedEntryCopyWithImpl;
@override @useResult
$Res call({
 String questionId, DateTime createdAt
});




}
/// @nodoc
class __$SavedEntryCopyWithImpl<$Res>
    implements _$SavedEntryCopyWith<$Res> {
  __$SavedEntryCopyWithImpl(this._self, this._then);

  final _SavedEntry _self;
  final $Res Function(_SavedEntry) _then;

/// Create a copy of SavedEntry
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? questionId = null,Object? createdAt = null,}) {
  return _then(_SavedEntry(
questionId: null == questionId ? _self.questionId : questionId // ignore: cast_nullable_to_non_nullable
as String,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime,
  ));
}


}

// dart format on
