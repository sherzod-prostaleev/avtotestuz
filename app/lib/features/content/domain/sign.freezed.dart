// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'sign.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$Sign {

 String get code; String get name; String? get groupCode; String? get description; String? get imageUrl; int? get questionCount; List<String>? get questionIds;
/// Create a copy of Sign
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SignCopyWith<Sign> get copyWith => _$SignCopyWithImpl<Sign>(this as Sign, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Sign&&(identical(other.code, code) || other.code == code)&&(identical(other.name, name) || other.name == name)&&(identical(other.groupCode, groupCode) || other.groupCode == groupCode)&&(identical(other.description, description) || other.description == description)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.questionCount, questionCount) || other.questionCount == questionCount)&&const DeepCollectionEquality().equals(other.questionIds, questionIds));
}


@override
int get hashCode => Object.hash(runtimeType,code,name,groupCode,description,imageUrl,questionCount,const DeepCollectionEquality().hash(questionIds));

@override
String toString() {
  return 'Sign(code: $code, name: $name, groupCode: $groupCode, description: $description, imageUrl: $imageUrl, questionCount: $questionCount, questionIds: $questionIds)';
}


}

/// @nodoc
abstract mixin class $SignCopyWith<$Res>  {
  factory $SignCopyWith(Sign value, $Res Function(Sign) _then) = _$SignCopyWithImpl;
@useResult
$Res call({
 String code, String name, String? groupCode, String? description, String? imageUrl, int? questionCount, List<String>? questionIds
});




}
/// @nodoc
class _$SignCopyWithImpl<$Res>
    implements $SignCopyWith<$Res> {
  _$SignCopyWithImpl(this._self, this._then);

  final Sign _self;
  final $Res Function(Sign) _then;

/// Create a copy of Sign
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? code = null,Object? name = null,Object? groupCode = freezed,Object? description = freezed,Object? imageUrl = freezed,Object? questionCount = freezed,Object? questionIds = freezed,}) {
  return _then(_self.copyWith(
code: null == code ? _self.code : code // ignore: cast_nullable_to_non_nullable
as String,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,groupCode: freezed == groupCode ? _self.groupCode : groupCode // ignore: cast_nullable_to_non_nullable
as String?,description: freezed == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String?,imageUrl: freezed == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String?,questionCount: freezed == questionCount ? _self.questionCount : questionCount // ignore: cast_nullable_to_non_nullable
as int?,questionIds: freezed == questionIds ? _self.questionIds : questionIds // ignore: cast_nullable_to_non_nullable
as List<String>?,
  ));
}

}


/// Adds pattern-matching-related methods to [Sign].
extension SignPatterns on Sign {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Sign value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Sign() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Sign value)  $default,){
final _that = this;
switch (_that) {
case _Sign():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Sign value)?  $default,){
final _that = this;
switch (_that) {
case _Sign() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String code,  String name,  String? groupCode,  String? description,  String? imageUrl,  int? questionCount,  List<String>? questionIds)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Sign() when $default != null:
return $default(_that.code,_that.name,_that.groupCode,_that.description,_that.imageUrl,_that.questionCount,_that.questionIds);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String code,  String name,  String? groupCode,  String? description,  String? imageUrl,  int? questionCount,  List<String>? questionIds)  $default,) {final _that = this;
switch (_that) {
case _Sign():
return $default(_that.code,_that.name,_that.groupCode,_that.description,_that.imageUrl,_that.questionCount,_that.questionIds);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String code,  String name,  String? groupCode,  String? description,  String? imageUrl,  int? questionCount,  List<String>? questionIds)?  $default,) {final _that = this;
switch (_that) {
case _Sign() when $default != null:
return $default(_that.code,_that.name,_that.groupCode,_that.description,_that.imageUrl,_that.questionCount,_that.questionIds);case _:
  return null;

}
}

}

/// @nodoc


class _Sign implements Sign {
  const _Sign({required this.code, required this.name, this.groupCode, this.description, this.imageUrl, this.questionCount, final  List<String>? questionIds}): _questionIds = questionIds;
  

@override final  String code;
@override final  String name;
@override final  String? groupCode;
@override final  String? description;
@override final  String? imageUrl;
@override final  int? questionCount;
 final  List<String>? _questionIds;
@override List<String>? get questionIds {
  final value = _questionIds;
  if (value == null) return null;
  if (_questionIds is EqualUnmodifiableListView) return _questionIds;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(value);
}


/// Create a copy of Sign
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SignCopyWith<_Sign> get copyWith => __$SignCopyWithImpl<_Sign>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Sign&&(identical(other.code, code) || other.code == code)&&(identical(other.name, name) || other.name == name)&&(identical(other.groupCode, groupCode) || other.groupCode == groupCode)&&(identical(other.description, description) || other.description == description)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&(identical(other.questionCount, questionCount) || other.questionCount == questionCount)&&const DeepCollectionEquality().equals(other._questionIds, _questionIds));
}


@override
int get hashCode => Object.hash(runtimeType,code,name,groupCode,description,imageUrl,questionCount,const DeepCollectionEquality().hash(_questionIds));

@override
String toString() {
  return 'Sign(code: $code, name: $name, groupCode: $groupCode, description: $description, imageUrl: $imageUrl, questionCount: $questionCount, questionIds: $questionIds)';
}


}

/// @nodoc
abstract mixin class _$SignCopyWith<$Res> implements $SignCopyWith<$Res> {
  factory _$SignCopyWith(_Sign value, $Res Function(_Sign) _then) = __$SignCopyWithImpl;
@override @useResult
$Res call({
 String code, String name, String? groupCode, String? description, String? imageUrl, int? questionCount, List<String>? questionIds
});




}
/// @nodoc
class __$SignCopyWithImpl<$Res>
    implements _$SignCopyWith<$Res> {
  __$SignCopyWithImpl(this._self, this._then);

  final _Sign _self;
  final $Res Function(_Sign) _then;

/// Create a copy of Sign
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? code = null,Object? name = null,Object? groupCode = freezed,Object? description = freezed,Object? imageUrl = freezed,Object? questionCount = freezed,Object? questionIds = freezed,}) {
  return _then(_Sign(
code: null == code ? _self.code : code // ignore: cast_nullable_to_non_nullable
as String,name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,groupCode: freezed == groupCode ? _self.groupCode : groupCode // ignore: cast_nullable_to_non_nullable
as String?,description: freezed == description ? _self.description : description // ignore: cast_nullable_to_non_nullable
as String?,imageUrl: freezed == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String?,questionCount: freezed == questionCount ? _self.questionCount : questionCount // ignore: cast_nullable_to_non_nullable
as int?,questionIds: freezed == questionIds ? _self._questionIds : questionIds // ignore: cast_nullable_to_non_nullable
as List<String>?,
  ));
}


}

// dart format on
