// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'question.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$Answer {

 String get id; int get position; String get text; String? get imageUrl;
/// Create a copy of Answer
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AnswerCopyWith<Answer> get copyWith => _$AnswerCopyWithImpl<Answer>(this as Answer, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Answer&&(identical(other.id, id) || other.id == id)&&(identical(other.position, position) || other.position == position)&&(identical(other.text, text) || other.text == text)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl));
}


@override
int get hashCode => Object.hash(runtimeType,id,position,text,imageUrl);

@override
String toString() {
  return 'Answer(id: $id, position: $position, text: $text, imageUrl: $imageUrl)';
}


}

/// @nodoc
abstract mixin class $AnswerCopyWith<$Res>  {
  factory $AnswerCopyWith(Answer value, $Res Function(Answer) _then) = _$AnswerCopyWithImpl;
@useResult
$Res call({
 String id, int position, String text, String? imageUrl
});




}
/// @nodoc
class _$AnswerCopyWithImpl<$Res>
    implements $AnswerCopyWith<$Res> {
  _$AnswerCopyWithImpl(this._self, this._then);

  final Answer _self;
  final $Res Function(Answer) _then;

/// Create a copy of Answer
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? position = null,Object? text = null,Object? imageUrl = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,position: null == position ? _self.position : position // ignore: cast_nullable_to_non_nullable
as int,text: null == text ? _self.text : text // ignore: cast_nullable_to_non_nullable
as String,imageUrl: freezed == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

}


/// Adds pattern-matching-related methods to [Answer].
extension AnswerPatterns on Answer {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Answer value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Answer() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Answer value)  $default,){
final _that = this;
switch (_that) {
case _Answer():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Answer value)?  $default,){
final _that = this;
switch (_that) {
case _Answer() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  int position,  String text,  String? imageUrl)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Answer() when $default != null:
return $default(_that.id,_that.position,_that.text,_that.imageUrl);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  int position,  String text,  String? imageUrl)  $default,) {final _that = this;
switch (_that) {
case _Answer():
return $default(_that.id,_that.position,_that.text,_that.imageUrl);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  int position,  String text,  String? imageUrl)?  $default,) {final _that = this;
switch (_that) {
case _Answer() when $default != null:
return $default(_that.id,_that.position,_that.text,_that.imageUrl);case _:
  return null;

}
}

}

/// @nodoc


class _Answer implements Answer {
  const _Answer({required this.id, required this.position, required this.text, this.imageUrl});
  

@override final  String id;
@override final  int position;
@override final  String text;
@override final  String? imageUrl;

/// Create a copy of Answer
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AnswerCopyWith<_Answer> get copyWith => __$AnswerCopyWithImpl<_Answer>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Answer&&(identical(other.id, id) || other.id == id)&&(identical(other.position, position) || other.position == position)&&(identical(other.text, text) || other.text == text)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl));
}


@override
int get hashCode => Object.hash(runtimeType,id,position,text,imageUrl);

@override
String toString() {
  return 'Answer(id: $id, position: $position, text: $text, imageUrl: $imageUrl)';
}


}

/// @nodoc
abstract mixin class _$AnswerCopyWith<$Res> implements $AnswerCopyWith<$Res> {
  factory _$AnswerCopyWith(_Answer value, $Res Function(_Answer) _then) = __$AnswerCopyWithImpl;
@override @useResult
$Res call({
 String id, int position, String text, String? imageUrl
});




}
/// @nodoc
class __$AnswerCopyWithImpl<$Res>
    implements _$AnswerCopyWith<$Res> {
  __$AnswerCopyWithImpl(this._self, this._then);

  final _Answer _self;
  final $Res Function(_Answer) _then;

/// Create a copy of Answer
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? position = null,Object? text = null,Object? imageUrl = freezed,}) {
  return _then(_Answer(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,position: null == position ? _self.position : position // ignore: cast_nullable_to_non_nullable
as int,text: null == text ? _self.text : text // ignore: cast_nullable_to_non_nullable
as String,imageUrl: freezed == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}

/// @nodoc
mixin _$LegalRef {

 String get code; String get title;
/// Create a copy of LegalRef
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$LegalRefCopyWith<LegalRef> get copyWith => _$LegalRefCopyWithImpl<LegalRef>(this as LegalRef, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is LegalRef&&(identical(other.code, code) || other.code == code)&&(identical(other.title, title) || other.title == title));
}


@override
int get hashCode => Object.hash(runtimeType,code,title);

@override
String toString() {
  return 'LegalRef(code: $code, title: $title)';
}


}

/// @nodoc
abstract mixin class $LegalRefCopyWith<$Res>  {
  factory $LegalRefCopyWith(LegalRef value, $Res Function(LegalRef) _then) = _$LegalRefCopyWithImpl;
@useResult
$Res call({
 String code, String title
});




}
/// @nodoc
class _$LegalRefCopyWithImpl<$Res>
    implements $LegalRefCopyWith<$Res> {
  _$LegalRefCopyWithImpl(this._self, this._then);

  final LegalRef _self;
  final $Res Function(LegalRef) _then;

/// Create a copy of LegalRef
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? code = null,Object? title = null,}) {
  return _then(_self.copyWith(
code: null == code ? _self.code : code // ignore: cast_nullable_to_non_nullable
as String,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [LegalRef].
extension LegalRefPatterns on LegalRef {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _LegalRef value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _LegalRef() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _LegalRef value)  $default,){
final _that = this;
switch (_that) {
case _LegalRef():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _LegalRef value)?  $default,){
final _that = this;
switch (_that) {
case _LegalRef() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String code,  String title)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _LegalRef() when $default != null:
return $default(_that.code,_that.title);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String code,  String title)  $default,) {final _that = this;
switch (_that) {
case _LegalRef():
return $default(_that.code,_that.title);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String code,  String title)?  $default,) {final _that = this;
switch (_that) {
case _LegalRef() when $default != null:
return $default(_that.code,_that.title);case _:
  return null;

}
}

}

/// @nodoc


class _LegalRef implements LegalRef {
  const _LegalRef({required this.code, required this.title});
  

@override final  String code;
@override final  String title;

/// Create a copy of LegalRef
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$LegalRefCopyWith<_LegalRef> get copyWith => __$LegalRefCopyWithImpl<_LegalRef>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _LegalRef&&(identical(other.code, code) || other.code == code)&&(identical(other.title, title) || other.title == title));
}


@override
int get hashCode => Object.hash(runtimeType,code,title);

@override
String toString() {
  return 'LegalRef(code: $code, title: $title)';
}


}

/// @nodoc
abstract mixin class _$LegalRefCopyWith<$Res> implements $LegalRefCopyWith<$Res> {
  factory _$LegalRefCopyWith(_LegalRef value, $Res Function(_LegalRef) _then) = __$LegalRefCopyWithImpl;
@override @useResult
$Res call({
 String code, String title
});




}
/// @nodoc
class __$LegalRefCopyWithImpl<$Res>
    implements _$LegalRefCopyWith<$Res> {
  __$LegalRefCopyWithImpl(this._self, this._then);

  final _LegalRef _self;
  final $Res Function(_LegalRef) _then;

/// Create a copy of LegalRef
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? code = null,Object? title = null,}) {
  return _then(_LegalRef(
code: null == code ? _self.code : code // ignore: cast_nullable_to_non_nullable
as String,title: null == title ? _self.title : title // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

/// @nodoc
mixin _$AnswerAnalysisItem {

 int get position; bool get correct; String get text;
/// Create a copy of AnswerAnalysisItem
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AnswerAnalysisItemCopyWith<AnswerAnalysisItem> get copyWith => _$AnswerAnalysisItemCopyWithImpl<AnswerAnalysisItem>(this as AnswerAnalysisItem, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AnswerAnalysisItem&&(identical(other.position, position) || other.position == position)&&(identical(other.correct, correct) || other.correct == correct)&&(identical(other.text, text) || other.text == text));
}


@override
int get hashCode => Object.hash(runtimeType,position,correct,text);

@override
String toString() {
  return 'AnswerAnalysisItem(position: $position, correct: $correct, text: $text)';
}


}

/// @nodoc
abstract mixin class $AnswerAnalysisItemCopyWith<$Res>  {
  factory $AnswerAnalysisItemCopyWith(AnswerAnalysisItem value, $Res Function(AnswerAnalysisItem) _then) = _$AnswerAnalysisItemCopyWithImpl;
@useResult
$Res call({
 int position, bool correct, String text
});




}
/// @nodoc
class _$AnswerAnalysisItemCopyWithImpl<$Res>
    implements $AnswerAnalysisItemCopyWith<$Res> {
  _$AnswerAnalysisItemCopyWithImpl(this._self, this._then);

  final AnswerAnalysisItem _self;
  final $Res Function(AnswerAnalysisItem) _then;

/// Create a copy of AnswerAnalysisItem
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? position = null,Object? correct = null,Object? text = null,}) {
  return _then(_self.copyWith(
position: null == position ? _self.position : position // ignore: cast_nullable_to_non_nullable
as int,correct: null == correct ? _self.correct : correct // ignore: cast_nullable_to_non_nullable
as bool,text: null == text ? _self.text : text // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [AnswerAnalysisItem].
extension AnswerAnalysisItemPatterns on AnswerAnalysisItem {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _AnswerAnalysisItem value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _AnswerAnalysisItem() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _AnswerAnalysisItem value)  $default,){
final _that = this;
switch (_that) {
case _AnswerAnalysisItem():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _AnswerAnalysisItem value)?  $default,){
final _that = this;
switch (_that) {
case _AnswerAnalysisItem() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( int position,  bool correct,  String text)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AnswerAnalysisItem() when $default != null:
return $default(_that.position,_that.correct,_that.text);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( int position,  bool correct,  String text)  $default,) {final _that = this;
switch (_that) {
case _AnswerAnalysisItem():
return $default(_that.position,_that.correct,_that.text);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( int position,  bool correct,  String text)?  $default,) {final _that = this;
switch (_that) {
case _AnswerAnalysisItem() when $default != null:
return $default(_that.position,_that.correct,_that.text);case _:
  return null;

}
}

}

/// @nodoc


class _AnswerAnalysisItem implements AnswerAnalysisItem {
  const _AnswerAnalysisItem({required this.position, required this.correct, required this.text});
  

@override final  int position;
@override final  bool correct;
@override final  String text;

/// Create a copy of AnswerAnalysisItem
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AnswerAnalysisItemCopyWith<_AnswerAnalysisItem> get copyWith => __$AnswerAnalysisItemCopyWithImpl<_AnswerAnalysisItem>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AnswerAnalysisItem&&(identical(other.position, position) || other.position == position)&&(identical(other.correct, correct) || other.correct == correct)&&(identical(other.text, text) || other.text == text));
}


@override
int get hashCode => Object.hash(runtimeType,position,correct,text);

@override
String toString() {
  return 'AnswerAnalysisItem(position: $position, correct: $correct, text: $text)';
}


}

/// @nodoc
abstract mixin class _$AnswerAnalysisItemCopyWith<$Res> implements $AnswerAnalysisItemCopyWith<$Res> {
  factory _$AnswerAnalysisItemCopyWith(_AnswerAnalysisItem value, $Res Function(_AnswerAnalysisItem) _then) = __$AnswerAnalysisItemCopyWithImpl;
@override @useResult
$Res call({
 int position, bool correct, String text
});




}
/// @nodoc
class __$AnswerAnalysisItemCopyWithImpl<$Res>
    implements _$AnswerAnalysisItemCopyWith<$Res> {
  __$AnswerAnalysisItemCopyWithImpl(this._self, this._then);

  final _AnswerAnalysisItem _self;
  final $Res Function(_AnswerAnalysisItem) _then;

/// Create a copy of AnswerAnalysisItem
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? position = null,Object? correct = null,Object? text = null,}) {
  return _then(_AnswerAnalysisItem(
position: null == position ? _self.position : position // ignore: cast_nullable_to_non_nullable
as int,correct: null == correct ? _self.correct : correct // ignore: cast_nullable_to_non_nullable
as bool,text: null == text ? _self.text : text // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

/// @nodoc
mixin _$ExplanationBlock {

 String get type; String? get text; List<AnswerAnalysisItem> get items;
/// Create a copy of ExplanationBlock
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ExplanationBlockCopyWith<ExplanationBlock> get copyWith => _$ExplanationBlockCopyWithImpl<ExplanationBlock>(this as ExplanationBlock, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ExplanationBlock&&(identical(other.type, type) || other.type == type)&&(identical(other.text, text) || other.text == text)&&const DeepCollectionEquality().equals(other.items, items));
}


@override
int get hashCode => Object.hash(runtimeType,type,text,const DeepCollectionEquality().hash(items));

@override
String toString() {
  return 'ExplanationBlock(type: $type, text: $text, items: $items)';
}


}

/// @nodoc
abstract mixin class $ExplanationBlockCopyWith<$Res>  {
  factory $ExplanationBlockCopyWith(ExplanationBlock value, $Res Function(ExplanationBlock) _then) = _$ExplanationBlockCopyWithImpl;
@useResult
$Res call({
 String type, String? text, List<AnswerAnalysisItem> items
});




}
/// @nodoc
class _$ExplanationBlockCopyWithImpl<$Res>
    implements $ExplanationBlockCopyWith<$Res> {
  _$ExplanationBlockCopyWithImpl(this._self, this._then);

  final ExplanationBlock _self;
  final $Res Function(ExplanationBlock) _then;

/// Create a copy of ExplanationBlock
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? type = null,Object? text = freezed,Object? items = null,}) {
  return _then(_self.copyWith(
type: null == type ? _self.type : type // ignore: cast_nullable_to_non_nullable
as String,text: freezed == text ? _self.text : text // ignore: cast_nullable_to_non_nullable
as String?,items: null == items ? _self.items : items // ignore: cast_nullable_to_non_nullable
as List<AnswerAnalysisItem>,
  ));
}

}


/// Adds pattern-matching-related methods to [ExplanationBlock].
extension ExplanationBlockPatterns on ExplanationBlock {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _ExplanationBlock value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _ExplanationBlock() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _ExplanationBlock value)  $default,){
final _that = this;
switch (_that) {
case _ExplanationBlock():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _ExplanationBlock value)?  $default,){
final _that = this;
switch (_that) {
case _ExplanationBlock() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String type,  String? text,  List<AnswerAnalysisItem> items)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _ExplanationBlock() when $default != null:
return $default(_that.type,_that.text,_that.items);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String type,  String? text,  List<AnswerAnalysisItem> items)  $default,) {final _that = this;
switch (_that) {
case _ExplanationBlock():
return $default(_that.type,_that.text,_that.items);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String type,  String? text,  List<AnswerAnalysisItem> items)?  $default,) {final _that = this;
switch (_that) {
case _ExplanationBlock() when $default != null:
return $default(_that.type,_that.text,_that.items);case _:
  return null;

}
}

}

/// @nodoc


class _ExplanationBlock implements ExplanationBlock {
  const _ExplanationBlock({required this.type, this.text, final  List<AnswerAnalysisItem> items = const <AnswerAnalysisItem>[]}): _items = items;
  

@override final  String type;
@override final  String? text;
 final  List<AnswerAnalysisItem> _items;
@override@JsonKey() List<AnswerAnalysisItem> get items {
  if (_items is EqualUnmodifiableListView) return _items;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_items);
}


/// Create a copy of ExplanationBlock
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ExplanationBlockCopyWith<_ExplanationBlock> get copyWith => __$ExplanationBlockCopyWithImpl<_ExplanationBlock>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _ExplanationBlock&&(identical(other.type, type) || other.type == type)&&(identical(other.text, text) || other.text == text)&&const DeepCollectionEquality().equals(other._items, _items));
}


@override
int get hashCode => Object.hash(runtimeType,type,text,const DeepCollectionEquality().hash(_items));

@override
String toString() {
  return 'ExplanationBlock(type: $type, text: $text, items: $items)';
}


}

/// @nodoc
abstract mixin class _$ExplanationBlockCopyWith<$Res> implements $ExplanationBlockCopyWith<$Res> {
  factory _$ExplanationBlockCopyWith(_ExplanationBlock value, $Res Function(_ExplanationBlock) _then) = __$ExplanationBlockCopyWithImpl;
@override @useResult
$Res call({
 String type, String? text, List<AnswerAnalysisItem> items
});




}
/// @nodoc
class __$ExplanationBlockCopyWithImpl<$Res>
    implements _$ExplanationBlockCopyWith<$Res> {
  __$ExplanationBlockCopyWithImpl(this._self, this._then);

  final _ExplanationBlock _self;
  final $Res Function(_ExplanationBlock) _then;

/// Create a copy of ExplanationBlock
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? type = null,Object? text = freezed,Object? items = null,}) {
  return _then(_ExplanationBlock(
type: null == type ? _self.type : type // ignore: cast_nullable_to_non_nullable
as String,text: freezed == text ? _self.text : text // ignore: cast_nullable_to_non_nullable
as String?,items: null == items ? _self._items : items // ignore: cast_nullable_to_non_nullable
as List<AnswerAnalysisItem>,
  ));
}


}

/// @nodoc
mixin _$Explanation {

 List<LegalRef> get legalRefs; List<ExplanationBlock> get blocks;
/// Create a copy of Explanation
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ExplanationCopyWith<Explanation> get copyWith => _$ExplanationCopyWithImpl<Explanation>(this as Explanation, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Explanation&&const DeepCollectionEquality().equals(other.legalRefs, legalRefs)&&const DeepCollectionEquality().equals(other.blocks, blocks));
}


@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(legalRefs),const DeepCollectionEquality().hash(blocks));

@override
String toString() {
  return 'Explanation(legalRefs: $legalRefs, blocks: $blocks)';
}


}

/// @nodoc
abstract mixin class $ExplanationCopyWith<$Res>  {
  factory $ExplanationCopyWith(Explanation value, $Res Function(Explanation) _then) = _$ExplanationCopyWithImpl;
@useResult
$Res call({
 List<LegalRef> legalRefs, List<ExplanationBlock> blocks
});




}
/// @nodoc
class _$ExplanationCopyWithImpl<$Res>
    implements $ExplanationCopyWith<$Res> {
  _$ExplanationCopyWithImpl(this._self, this._then);

  final Explanation _self;
  final $Res Function(Explanation) _then;

/// Create a copy of Explanation
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? legalRefs = null,Object? blocks = null,}) {
  return _then(_self.copyWith(
legalRefs: null == legalRefs ? _self.legalRefs : legalRefs // ignore: cast_nullable_to_non_nullable
as List<LegalRef>,blocks: null == blocks ? _self.blocks : blocks // ignore: cast_nullable_to_non_nullable
as List<ExplanationBlock>,
  ));
}

}


/// Adds pattern-matching-related methods to [Explanation].
extension ExplanationPatterns on Explanation {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Explanation value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Explanation() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Explanation value)  $default,){
final _that = this;
switch (_that) {
case _Explanation():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Explanation value)?  $default,){
final _that = this;
switch (_that) {
case _Explanation() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( List<LegalRef> legalRefs,  List<ExplanationBlock> blocks)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Explanation() when $default != null:
return $default(_that.legalRefs,_that.blocks);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( List<LegalRef> legalRefs,  List<ExplanationBlock> blocks)  $default,) {final _that = this;
switch (_that) {
case _Explanation():
return $default(_that.legalRefs,_that.blocks);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( List<LegalRef> legalRefs,  List<ExplanationBlock> blocks)?  $default,) {final _that = this;
switch (_that) {
case _Explanation() when $default != null:
return $default(_that.legalRefs,_that.blocks);case _:
  return null;

}
}

}

/// @nodoc


class _Explanation implements Explanation {
  const _Explanation({required final  List<LegalRef> legalRefs, required final  List<ExplanationBlock> blocks}): _legalRefs = legalRefs,_blocks = blocks;
  

 final  List<LegalRef> _legalRefs;
@override List<LegalRef> get legalRefs {
  if (_legalRefs is EqualUnmodifiableListView) return _legalRefs;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_legalRefs);
}

 final  List<ExplanationBlock> _blocks;
@override List<ExplanationBlock> get blocks {
  if (_blocks is EqualUnmodifiableListView) return _blocks;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_blocks);
}


/// Create a copy of Explanation
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ExplanationCopyWith<_Explanation> get copyWith => __$ExplanationCopyWithImpl<_Explanation>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Explanation&&const DeepCollectionEquality().equals(other._legalRefs, _legalRefs)&&const DeepCollectionEquality().equals(other._blocks, _blocks));
}


@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(_legalRefs),const DeepCollectionEquality().hash(_blocks));

@override
String toString() {
  return 'Explanation(legalRefs: $legalRefs, blocks: $blocks)';
}


}

/// @nodoc
abstract mixin class _$ExplanationCopyWith<$Res> implements $ExplanationCopyWith<$Res> {
  factory _$ExplanationCopyWith(_Explanation value, $Res Function(_Explanation) _then) = __$ExplanationCopyWithImpl;
@override @useResult
$Res call({
 List<LegalRef> legalRefs, List<ExplanationBlock> blocks
});




}
/// @nodoc
class __$ExplanationCopyWithImpl<$Res>
    implements _$ExplanationCopyWith<$Res> {
  __$ExplanationCopyWithImpl(this._self, this._then);

  final _Explanation _self;
  final $Res Function(_Explanation) _then;

/// Create a copy of Explanation
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? legalRefs = null,Object? blocks = null,}) {
  return _then(_Explanation(
legalRefs: null == legalRefs ? _self._legalRefs : legalRefs // ignore: cast_nullable_to_non_nullable
as List<LegalRef>,blocks: null == blocks ? _self._blocks : blocks // ignore: cast_nullable_to_non_nullable
as List<ExplanationBlock>,
  ));
}


}

/// @nodoc
mixin _$Question {

 String get id; int? get position; String get categoryCode; String get text; String? get imageUrl; List<Answer> get answers; List<Sign> get signs; Explanation? get explanation;
/// Create a copy of Question
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$QuestionCopyWith<Question> get copyWith => _$QuestionCopyWithImpl<Question>(this as Question, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is Question&&(identical(other.id, id) || other.id == id)&&(identical(other.position, position) || other.position == position)&&(identical(other.categoryCode, categoryCode) || other.categoryCode == categoryCode)&&(identical(other.text, text) || other.text == text)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&const DeepCollectionEquality().equals(other.answers, answers)&&const DeepCollectionEquality().equals(other.signs, signs)&&(identical(other.explanation, explanation) || other.explanation == explanation));
}


@override
int get hashCode => Object.hash(runtimeType,id,position,categoryCode,text,imageUrl,const DeepCollectionEquality().hash(answers),const DeepCollectionEquality().hash(signs),explanation);

@override
String toString() {
  return 'Question(id: $id, position: $position, categoryCode: $categoryCode, text: $text, imageUrl: $imageUrl, answers: $answers, signs: $signs, explanation: $explanation)';
}


}

/// @nodoc
abstract mixin class $QuestionCopyWith<$Res>  {
  factory $QuestionCopyWith(Question value, $Res Function(Question) _then) = _$QuestionCopyWithImpl;
@useResult
$Res call({
 String id, int? position, String categoryCode, String text, String? imageUrl, List<Answer> answers, List<Sign> signs, Explanation? explanation
});


$ExplanationCopyWith<$Res>? get explanation;

}
/// @nodoc
class _$QuestionCopyWithImpl<$Res>
    implements $QuestionCopyWith<$Res> {
  _$QuestionCopyWithImpl(this._self, this._then);

  final Question _self;
  final $Res Function(Question) _then;

/// Create a copy of Question
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? position = freezed,Object? categoryCode = null,Object? text = null,Object? imageUrl = freezed,Object? answers = null,Object? signs = null,Object? explanation = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,position: freezed == position ? _self.position : position // ignore: cast_nullable_to_non_nullable
as int?,categoryCode: null == categoryCode ? _self.categoryCode : categoryCode // ignore: cast_nullable_to_non_nullable
as String,text: null == text ? _self.text : text // ignore: cast_nullable_to_non_nullable
as String,imageUrl: freezed == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String?,answers: null == answers ? _self.answers : answers // ignore: cast_nullable_to_non_nullable
as List<Answer>,signs: null == signs ? _self.signs : signs // ignore: cast_nullable_to_non_nullable
as List<Sign>,explanation: freezed == explanation ? _self.explanation : explanation // ignore: cast_nullable_to_non_nullable
as Explanation?,
  ));
}
/// Create a copy of Question
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$ExplanationCopyWith<$Res>? get explanation {
    if (_self.explanation == null) {
    return null;
  }

  return $ExplanationCopyWith<$Res>(_self.explanation!, (value) {
    return _then(_self.copyWith(explanation: value));
  });
}
}


/// Adds pattern-matching-related methods to [Question].
extension QuestionPatterns on Question {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _Question value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _Question() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _Question value)  $default,){
final _that = this;
switch (_that) {
case _Question():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _Question value)?  $default,){
final _that = this;
switch (_that) {
case _Question() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  int? position,  String categoryCode,  String text,  String? imageUrl,  List<Answer> answers,  List<Sign> signs,  Explanation? explanation)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _Question() when $default != null:
return $default(_that.id,_that.position,_that.categoryCode,_that.text,_that.imageUrl,_that.answers,_that.signs,_that.explanation);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  int? position,  String categoryCode,  String text,  String? imageUrl,  List<Answer> answers,  List<Sign> signs,  Explanation? explanation)  $default,) {final _that = this;
switch (_that) {
case _Question():
return $default(_that.id,_that.position,_that.categoryCode,_that.text,_that.imageUrl,_that.answers,_that.signs,_that.explanation);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  int? position,  String categoryCode,  String text,  String? imageUrl,  List<Answer> answers,  List<Sign> signs,  Explanation? explanation)?  $default,) {final _that = this;
switch (_that) {
case _Question() when $default != null:
return $default(_that.id,_that.position,_that.categoryCode,_that.text,_that.imageUrl,_that.answers,_that.signs,_that.explanation);case _:
  return null;

}
}

}

/// @nodoc


class _Question implements Question {
  const _Question({required this.id, this.position, required this.categoryCode, required this.text, this.imageUrl, required final  List<Answer> answers, final  List<Sign> signs = const <Sign>[], this.explanation}): _answers = answers,_signs = signs;
  

@override final  String id;
@override final  int? position;
@override final  String categoryCode;
@override final  String text;
@override final  String? imageUrl;
 final  List<Answer> _answers;
@override List<Answer> get answers {
  if (_answers is EqualUnmodifiableListView) return _answers;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_answers);
}

 final  List<Sign> _signs;
@override@JsonKey() List<Sign> get signs {
  if (_signs is EqualUnmodifiableListView) return _signs;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_signs);
}

@override final  Explanation? explanation;

/// Create a copy of Question
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$QuestionCopyWith<_Question> get copyWith => __$QuestionCopyWithImpl<_Question>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _Question&&(identical(other.id, id) || other.id == id)&&(identical(other.position, position) || other.position == position)&&(identical(other.categoryCode, categoryCode) || other.categoryCode == categoryCode)&&(identical(other.text, text) || other.text == text)&&(identical(other.imageUrl, imageUrl) || other.imageUrl == imageUrl)&&const DeepCollectionEquality().equals(other._answers, _answers)&&const DeepCollectionEquality().equals(other._signs, _signs)&&(identical(other.explanation, explanation) || other.explanation == explanation));
}


@override
int get hashCode => Object.hash(runtimeType,id,position,categoryCode,text,imageUrl,const DeepCollectionEquality().hash(_answers),const DeepCollectionEquality().hash(_signs),explanation);

@override
String toString() {
  return 'Question(id: $id, position: $position, categoryCode: $categoryCode, text: $text, imageUrl: $imageUrl, answers: $answers, signs: $signs, explanation: $explanation)';
}


}

/// @nodoc
abstract mixin class _$QuestionCopyWith<$Res> implements $QuestionCopyWith<$Res> {
  factory _$QuestionCopyWith(_Question value, $Res Function(_Question) _then) = __$QuestionCopyWithImpl;
@override @useResult
$Res call({
 String id, int? position, String categoryCode, String text, String? imageUrl, List<Answer> answers, List<Sign> signs, Explanation? explanation
});


@override $ExplanationCopyWith<$Res>? get explanation;

}
/// @nodoc
class __$QuestionCopyWithImpl<$Res>
    implements _$QuestionCopyWith<$Res> {
  __$QuestionCopyWithImpl(this._self, this._then);

  final _Question _self;
  final $Res Function(_Question) _then;

/// Create a copy of Question
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? position = freezed,Object? categoryCode = null,Object? text = null,Object? imageUrl = freezed,Object? answers = null,Object? signs = null,Object? explanation = freezed,}) {
  return _then(_Question(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,position: freezed == position ? _self.position : position // ignore: cast_nullable_to_non_nullable
as int?,categoryCode: null == categoryCode ? _self.categoryCode : categoryCode // ignore: cast_nullable_to_non_nullable
as String,text: null == text ? _self.text : text // ignore: cast_nullable_to_non_nullable
as String,imageUrl: freezed == imageUrl ? _self.imageUrl : imageUrl // ignore: cast_nullable_to_non_nullable
as String?,answers: null == answers ? _self._answers : answers // ignore: cast_nullable_to_non_nullable
as List<Answer>,signs: null == signs ? _self._signs : signs // ignore: cast_nullable_to_non_nullable
as List<Sign>,explanation: freezed == explanation ? _self.explanation : explanation // ignore: cast_nullable_to_non_nullable
as Explanation?,
  ));
}

/// Create a copy of Question
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$ExplanationCopyWith<$Res>? get explanation {
    if (_self.explanation == null) {
    return null;
  }

  return $ExplanationCopyWith<$Res>(_self.explanation!, (value) {
    return _then(_self.copyWith(explanation: value));
  });
}
}

// dart format on
