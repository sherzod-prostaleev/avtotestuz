// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'variant.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$Variant {

 int get number; int? get questionCount; List<Question>? get questions;
/// Create a copy of Variant
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$VariantCopyWith<Variant> get copyWith => _$VariantCopyWithImpl<Variant>(this as Variant, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Variant&&(identical(other.number, number) || other.number == number)&&(identical(other.questionCount, questionCount) || other.questionCount == questionCount)&&const DeepCollectionEquality().equals(other.questions, questions));
}


@override
int get hashCode => Object.hash(runtimeType,number,questionCount,const DeepCollectionEquality().hash(questions));

@override
String toString() {
  return 'Variant(number: $number, questionCount: $questionCount, questions: $questions)';
}


}

/// @nodoc
abstract mixin class $VariantCopyWith<$Res>  {
  factory $VariantCopyWith(Variant value, $Res Function(Variant) _then) = _$VariantCopyWithImpl;
@useResult
$Res call({
 int number, int? questionCount, List<Question>? questions
});




}
/// @nodoc
class _$VariantCopyWithImpl<$Res>
    implements $VariantCopyWith<$Res> {
  _$VariantCopyWithImpl(this._self, this._then);

  final Variant _self;
  final $Res Function(Variant) _then;

/// Create a copy of Variant
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? number = null,Object? questionCount = freezed,Object? questions = freezed,}) {
  return _then(_self.copyWith(
number: null == number ? _self.number : number // ignore: cast_nullable_to_non_nullable
as int,questionCount: freezed == questionCount ? _self.questionCount : questionCount // ignore: cast_nullable_to_non_nullable
as int?,questions: freezed == questions ? _self.questions : questions // ignore: cast_nullable_to_non_nullable
as List<Question>?,
  ));
}

}


/// Adds pattern-matching-related methods to [Variant].
extension VariantPatterns on Variant {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Variant value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Variant() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Variant value)  $default,){
final _that = this;
switch (_that) {
case _Variant():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Variant value)?  $default,){
final _that = this;
switch (_that) {
case _Variant() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( int number,  int? questionCount,  List<Question>? questions)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Variant() when $default != null:
return $default(_that.number,_that.questionCount,_that.questions);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( int number,  int? questionCount,  List<Question>? questions)  $default,) {final _that = this;
switch (_that) {
case _Variant():
return $default(_that.number,_that.questionCount,_that.questions);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( int number,  int? questionCount,  List<Question>? questions)?  $default,) {final _that = this;
switch (_that) {
case _Variant() when $default != null:
return $default(_that.number,_that.questionCount,_that.questions);case _:
  return null;

}
}

}

/// @nodoc


class _Variant implements Variant {
  const _Variant({required this.number, this.questionCount, final  List<Question>? questions}): _questions = questions;
  

@override final  int number;
@override final  int? questionCount;
 final  List<Question>? _questions;
@override List<Question>? get questions {
  final value = _questions;
  if (value == null) return null;
  if (_questions is EqualUnmodifiableListView) return _questions;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(value);
}


/// Create a copy of Variant
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$VariantCopyWith<_Variant> get copyWith => __$VariantCopyWithImpl<_Variant>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Variant&&(identical(other.number, number) || other.number == number)&&(identical(other.questionCount, questionCount) || other.questionCount == questionCount)&&const DeepCollectionEquality().equals(other._questions, _questions));
}


@override
int get hashCode => Object.hash(runtimeType,number,questionCount,const DeepCollectionEquality().hash(_questions));

@override
String toString() {
  return 'Variant(number: $number, questionCount: $questionCount, questions: $questions)';
}


}

/// @nodoc
abstract mixin class _$VariantCopyWith<$Res> implements $VariantCopyWith<$Res> {
  factory _$VariantCopyWith(_Variant value, $Res Function(_Variant) _then) = __$VariantCopyWithImpl;
@override @useResult
$Res call({
 int number, int? questionCount, List<Question>? questions
});




}
/// @nodoc
class __$VariantCopyWithImpl<$Res>
    implements _$VariantCopyWith<$Res> {
  __$VariantCopyWithImpl(this._self, this._then);

  final _Variant _self;
  final $Res Function(_Variant) _then;

/// Create a copy of Variant
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? number = null,Object? questionCount = freezed,Object? questions = freezed,}) {
  return _then(_Variant(
number: null == number ? _self.number : number // ignore: cast_nullable_to_non_nullable
as int,questionCount: freezed == questionCount ? _self.questionCount : questionCount // ignore: cast_nullable_to_non_nullable
as int?,questions: freezed == questions ? _self._questions : questions // ignore: cast_nullable_to_non_nullable
as List<Question>?,
  ));
}


}

// dart format on
