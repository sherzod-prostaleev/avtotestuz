// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'session_models.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$SessionSummary {

 String get id; String get mode; List<String> get questionIds; int get timeLimitSec; int get total; DateTime get startedAt;
/// Create a copy of SessionSummary
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SessionSummaryCopyWith<SessionSummary> get copyWith => _$SessionSummaryCopyWithImpl<SessionSummary>(this as SessionSummary, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SessionSummary&&(identical(other.id, id) || other.id == id)&&(identical(other.mode, mode) || other.mode == mode)&&const DeepCollectionEquality().equals(other.questionIds, questionIds)&&(identical(other.timeLimitSec, timeLimitSec) || other.timeLimitSec == timeLimitSec)&&(identical(other.total, total) || other.total == total)&&(identical(other.startedAt, startedAt) || other.startedAt == startedAt));
}


@override
int get hashCode => Object.hash(runtimeType,id,mode,const DeepCollectionEquality().hash(questionIds),timeLimitSec,total,startedAt);

@override
String toString() {
  return 'SessionSummary(id: $id, mode: $mode, questionIds: $questionIds, timeLimitSec: $timeLimitSec, total: $total, startedAt: $startedAt)';
}


}

/// @nodoc
abstract mixin class $SessionSummaryCopyWith<$Res>  {
  factory $SessionSummaryCopyWith(SessionSummary value, $Res Function(SessionSummary) _then) = _$SessionSummaryCopyWithImpl;
@useResult
$Res call({
 String id, String mode, List<String> questionIds, int timeLimitSec, int total, DateTime startedAt
});




}
/// @nodoc
class _$SessionSummaryCopyWithImpl<$Res>
    implements $SessionSummaryCopyWith<$Res> {
  _$SessionSummaryCopyWithImpl(this._self, this._then);

  final SessionSummary _self;
  final $Res Function(SessionSummary) _then;

/// Create a copy of SessionSummary
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? mode = null,Object? questionIds = null,Object? timeLimitSec = null,Object? total = null,Object? startedAt = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,mode: null == mode ? _self.mode : mode // ignore: cast_nullable_to_non_nullable
as String,questionIds: null == questionIds ? _self.questionIds : questionIds // ignore: cast_nullable_to_non_nullable
as List<String>,timeLimitSec: null == timeLimitSec ? _self.timeLimitSec : timeLimitSec // ignore: cast_nullable_to_non_nullable
as int,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,startedAt: null == startedAt ? _self.startedAt : startedAt // ignore: cast_nullable_to_non_nullable
as DateTime,
  ));
}

}


/// Adds pattern-matching-related methods to [SessionSummary].
extension SessionSummaryPatterns on SessionSummary {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SessionSummary value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SessionSummary() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SessionSummary value)  $default,){
final _that = this;
switch (_that) {
case _SessionSummary():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SessionSummary value)?  $default,){
final _that = this;
switch (_that) {
case _SessionSummary() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String mode,  List<String> questionIds,  int timeLimitSec,  int total,  DateTime startedAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SessionSummary() when $default != null:
return $default(_that.id,_that.mode,_that.questionIds,_that.timeLimitSec,_that.total,_that.startedAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String mode,  List<String> questionIds,  int timeLimitSec,  int total,  DateTime startedAt)  $default,) {final _that = this;
switch (_that) {
case _SessionSummary():
return $default(_that.id,_that.mode,_that.questionIds,_that.timeLimitSec,_that.total,_that.startedAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String mode,  List<String> questionIds,  int timeLimitSec,  int total,  DateTime startedAt)?  $default,) {final _that = this;
switch (_that) {
case _SessionSummary() when $default != null:
return $default(_that.id,_that.mode,_that.questionIds,_that.timeLimitSec,_that.total,_that.startedAt);case _:
  return null;

}
}

}

/// @nodoc


class _SessionSummary implements SessionSummary {
  const _SessionSummary({required this.id, required this.mode, required final  List<String> questionIds, required this.timeLimitSec, required this.total, required this.startedAt}): _questionIds = questionIds;
  

@override final  String id;
@override final  String mode;
 final  List<String> _questionIds;
@override List<String> get questionIds {
  if (_questionIds is EqualUnmodifiableListView) return _questionIds;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_questionIds);
}

@override final  int timeLimitSec;
@override final  int total;
@override final  DateTime startedAt;

/// Create a copy of SessionSummary
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SessionSummaryCopyWith<_SessionSummary> get copyWith => __$SessionSummaryCopyWithImpl<_SessionSummary>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SessionSummary&&(identical(other.id, id) || other.id == id)&&(identical(other.mode, mode) || other.mode == mode)&&const DeepCollectionEquality().equals(other._questionIds, _questionIds)&&(identical(other.timeLimitSec, timeLimitSec) || other.timeLimitSec == timeLimitSec)&&(identical(other.total, total) || other.total == total)&&(identical(other.startedAt, startedAt) || other.startedAt == startedAt));
}


@override
int get hashCode => Object.hash(runtimeType,id,mode,const DeepCollectionEquality().hash(_questionIds),timeLimitSec,total,startedAt);

@override
String toString() {
  return 'SessionSummary(id: $id, mode: $mode, questionIds: $questionIds, timeLimitSec: $timeLimitSec, total: $total, startedAt: $startedAt)';
}


}

/// @nodoc
abstract mixin class _$SessionSummaryCopyWith<$Res> implements $SessionSummaryCopyWith<$Res> {
  factory _$SessionSummaryCopyWith(_SessionSummary value, $Res Function(_SessionSummary) _then) = __$SessionSummaryCopyWithImpl;
@override @useResult
$Res call({
 String id, String mode, List<String> questionIds, int timeLimitSec, int total, DateTime startedAt
});




}
/// @nodoc
class __$SessionSummaryCopyWithImpl<$Res>
    implements _$SessionSummaryCopyWith<$Res> {
  __$SessionSummaryCopyWithImpl(this._self, this._then);

  final _SessionSummary _self;
  final $Res Function(_SessionSummary) _then;

/// Create a copy of SessionSummary
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? mode = null,Object? questionIds = null,Object? timeLimitSec = null,Object? total = null,Object? startedAt = null,}) {
  return _then(_SessionSummary(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,mode: null == mode ? _self.mode : mode // ignore: cast_nullable_to_non_nullable
as String,questionIds: null == questionIds ? _self._questionIds : questionIds // ignore: cast_nullable_to_non_nullable
as List<String>,timeLimitSec: null == timeLimitSec ? _self.timeLimitSec : timeLimitSec // ignore: cast_nullable_to_non_nullable
as int,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,startedAt: null == startedAt ? _self.startedAt : startedAt // ignore: cast_nullable_to_non_nullable
as DateTime,
  ));
}


}

/// @nodoc
mixin _$AnswerResult {

 bool get recorded; bool? get correct; String? get correctAnswerId; bool get stopped; String? get stopReason;
/// Create a copy of AnswerResult
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AnswerResultCopyWith<AnswerResult> get copyWith => _$AnswerResultCopyWithImpl<AnswerResult>(this as AnswerResult, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AnswerResult&&(identical(other.recorded, recorded) || other.recorded == recorded)&&(identical(other.correct, correct) || other.correct == correct)&&(identical(other.correctAnswerId, correctAnswerId) || other.correctAnswerId == correctAnswerId)&&(identical(other.stopped, stopped) || other.stopped == stopped)&&(identical(other.stopReason, stopReason) || other.stopReason == stopReason));
}


@override
int get hashCode => Object.hash(runtimeType,recorded,correct,correctAnswerId,stopped,stopReason);

@override
String toString() {
  return 'AnswerResult(recorded: $recorded, correct: $correct, correctAnswerId: $correctAnswerId, stopped: $stopped, stopReason: $stopReason)';
}


}

/// @nodoc
abstract mixin class $AnswerResultCopyWith<$Res>  {
  factory $AnswerResultCopyWith(AnswerResult value, $Res Function(AnswerResult) _then) = _$AnswerResultCopyWithImpl;
@useResult
$Res call({
 bool recorded, bool? correct, String? correctAnswerId, bool stopped, String? stopReason
});




}
/// @nodoc
class _$AnswerResultCopyWithImpl<$Res>
    implements $AnswerResultCopyWith<$Res> {
  _$AnswerResultCopyWithImpl(this._self, this._then);

  final AnswerResult _self;
  final $Res Function(AnswerResult) _then;

/// Create a copy of AnswerResult
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? recorded = null,Object? correct = freezed,Object? correctAnswerId = freezed,Object? stopped = null,Object? stopReason = freezed,}) {
  return _then(_self.copyWith(
recorded: null == recorded ? _self.recorded : recorded // ignore: cast_nullable_to_non_nullable
as bool,correct: freezed == correct ? _self.correct : correct // ignore: cast_nullable_to_non_nullable
as bool?,correctAnswerId: freezed == correctAnswerId ? _self.correctAnswerId : correctAnswerId // ignore: cast_nullable_to_non_nullable
as String?,stopped: null == stopped ? _self.stopped : stopped // ignore: cast_nullable_to_non_nullable
as bool,stopReason: freezed == stopReason ? _self.stopReason : stopReason // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

}


/// Adds pattern-matching-related methods to [AnswerResult].
extension AnswerResultPatterns on AnswerResult {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _AnswerResult value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _AnswerResult() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _AnswerResult value)  $default,){
final _that = this;
switch (_that) {
case _AnswerResult():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _AnswerResult value)?  $default,){
final _that = this;
switch (_that) {
case _AnswerResult() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( bool recorded,  bool? correct,  String? correctAnswerId,  bool stopped,  String? stopReason)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AnswerResult() when $default != null:
return $default(_that.recorded,_that.correct,_that.correctAnswerId,_that.stopped,_that.stopReason);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( bool recorded,  bool? correct,  String? correctAnswerId,  bool stopped,  String? stopReason)  $default,) {final _that = this;
switch (_that) {
case _AnswerResult():
return $default(_that.recorded,_that.correct,_that.correctAnswerId,_that.stopped,_that.stopReason);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( bool recorded,  bool? correct,  String? correctAnswerId,  bool stopped,  String? stopReason)?  $default,) {final _that = this;
switch (_that) {
case _AnswerResult() when $default != null:
return $default(_that.recorded,_that.correct,_that.correctAnswerId,_that.stopped,_that.stopReason);case _:
  return null;

}
}

}

/// @nodoc


class _AnswerResult implements AnswerResult {
  const _AnswerResult({required this.recorded, this.correct, this.correctAnswerId, this.stopped = false, this.stopReason});
  

@override final  bool recorded;
@override final  bool? correct;
@override final  String? correctAnswerId;
@override@JsonKey() final  bool stopped;
@override final  String? stopReason;

/// Create a copy of AnswerResult
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AnswerResultCopyWith<_AnswerResult> get copyWith => __$AnswerResultCopyWithImpl<_AnswerResult>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AnswerResult&&(identical(other.recorded, recorded) || other.recorded == recorded)&&(identical(other.correct, correct) || other.correct == correct)&&(identical(other.correctAnswerId, correctAnswerId) || other.correctAnswerId == correctAnswerId)&&(identical(other.stopped, stopped) || other.stopped == stopped)&&(identical(other.stopReason, stopReason) || other.stopReason == stopReason));
}


@override
int get hashCode => Object.hash(runtimeType,recorded,correct,correctAnswerId,stopped,stopReason);

@override
String toString() {
  return 'AnswerResult(recorded: $recorded, correct: $correct, correctAnswerId: $correctAnswerId, stopped: $stopped, stopReason: $stopReason)';
}


}

/// @nodoc
abstract mixin class _$AnswerResultCopyWith<$Res> implements $AnswerResultCopyWith<$Res> {
  factory _$AnswerResultCopyWith(_AnswerResult value, $Res Function(_AnswerResult) _then) = __$AnswerResultCopyWithImpl;
@override @useResult
$Res call({
 bool recorded, bool? correct, String? correctAnswerId, bool stopped, String? stopReason
});




}
/// @nodoc
class __$AnswerResultCopyWithImpl<$Res>
    implements _$AnswerResultCopyWith<$Res> {
  __$AnswerResultCopyWithImpl(this._self, this._then);

  final _AnswerResult _self;
  final $Res Function(_AnswerResult) _then;

/// Create a copy of AnswerResult
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? recorded = null,Object? correct = freezed,Object? correctAnswerId = freezed,Object? stopped = null,Object? stopReason = freezed,}) {
  return _then(_AnswerResult(
recorded: null == recorded ? _self.recorded : recorded // ignore: cast_nullable_to_non_nullable
as bool,correct: freezed == correct ? _self.correct : correct // ignore: cast_nullable_to_non_nullable
as bool?,correctAnswerId: freezed == correctAnswerId ? _self.correctAnswerId : correctAnswerId // ignore: cast_nullable_to_non_nullable
as String?,stopped: null == stopped ? _self.stopped : stopped // ignore: cast_nullable_to_non_nullable
as bool,stopReason: freezed == stopReason ? _self.stopReason : stopReason // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}

/// @nodoc
mixin _$SessionResult {

 String get status; String get stoppedReason; int get score; int get total;
/// Create a copy of SessionResult
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SessionResultCopyWith<SessionResult> get copyWith => _$SessionResultCopyWithImpl<SessionResult>(this as SessionResult, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SessionResult&&(identical(other.status, status) || other.status == status)&&(identical(other.stoppedReason, stoppedReason) || other.stoppedReason == stoppedReason)&&(identical(other.score, score) || other.score == score)&&(identical(other.total, total) || other.total == total));
}


@override
int get hashCode => Object.hash(runtimeType,status,stoppedReason,score,total);

@override
String toString() {
  return 'SessionResult(status: $status, stoppedReason: $stoppedReason, score: $score, total: $total)';
}


}

/// @nodoc
abstract mixin class $SessionResultCopyWith<$Res>  {
  factory $SessionResultCopyWith(SessionResult value, $Res Function(SessionResult) _then) = _$SessionResultCopyWithImpl;
@useResult
$Res call({
 String status, String stoppedReason, int score, int total
});




}
/// @nodoc
class _$SessionResultCopyWithImpl<$Res>
    implements $SessionResultCopyWith<$Res> {
  _$SessionResultCopyWithImpl(this._self, this._then);

  final SessionResult _self;
  final $Res Function(SessionResult) _then;

/// Create a copy of SessionResult
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? status = null,Object? stoppedReason = null,Object? score = null,Object? total = null,}) {
  return _then(_self.copyWith(
status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,stoppedReason: null == stoppedReason ? _self.stoppedReason : stoppedReason // ignore: cast_nullable_to_non_nullable
as String,score: null == score ? _self.score : score // ignore: cast_nullable_to_non_nullable
as int,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [SessionResult].
extension SessionResultPatterns on SessionResult {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SessionResult value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SessionResult() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SessionResult value)  $default,){
final _that = this;
switch (_that) {
case _SessionResult():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SessionResult value)?  $default,){
final _that = this;
switch (_that) {
case _SessionResult() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String status,  String stoppedReason,  int score,  int total)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SessionResult() when $default != null:
return $default(_that.status,_that.stoppedReason,_that.score,_that.total);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String status,  String stoppedReason,  int score,  int total)  $default,) {final _that = this;
switch (_that) {
case _SessionResult():
return $default(_that.status,_that.stoppedReason,_that.score,_that.total);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String status,  String stoppedReason,  int score,  int total)?  $default,) {final _that = this;
switch (_that) {
case _SessionResult() when $default != null:
return $default(_that.status,_that.stoppedReason,_that.score,_that.total);case _:
  return null;

}
}

}

/// @nodoc


class _SessionResult implements SessionResult {
  const _SessionResult({required this.status, required this.stoppedReason, required this.score, required this.total});
  

@override final  String status;
@override final  String stoppedReason;
@override final  int score;
@override final  int total;

/// Create a copy of SessionResult
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SessionResultCopyWith<_SessionResult> get copyWith => __$SessionResultCopyWithImpl<_SessionResult>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SessionResult&&(identical(other.status, status) || other.status == status)&&(identical(other.stoppedReason, stoppedReason) || other.stoppedReason == stoppedReason)&&(identical(other.score, score) || other.score == score)&&(identical(other.total, total) || other.total == total));
}


@override
int get hashCode => Object.hash(runtimeType,status,stoppedReason,score,total);

@override
String toString() {
  return 'SessionResult(status: $status, stoppedReason: $stoppedReason, score: $score, total: $total)';
}


}

/// @nodoc
abstract mixin class _$SessionResultCopyWith<$Res> implements $SessionResultCopyWith<$Res> {
  factory _$SessionResultCopyWith(_SessionResult value, $Res Function(_SessionResult) _then) = __$SessionResultCopyWithImpl;
@override @useResult
$Res call({
 String status, String stoppedReason, int score, int total
});




}
/// @nodoc
class __$SessionResultCopyWithImpl<$Res>
    implements _$SessionResultCopyWith<$Res> {
  __$SessionResultCopyWithImpl(this._self, this._then);

  final _SessionResult _self;
  final $Res Function(_SessionResult) _then;

/// Create a copy of SessionResult
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? status = null,Object? stoppedReason = null,Object? score = null,Object? total = null,}) {
  return _then(_SessionResult(
status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,stoppedReason: null == stoppedReason ? _self.stoppedReason : stoppedReason // ignore: cast_nullable_to_non_nullable
as String,score: null == score ? _self.score : score // ignore: cast_nullable_to_non_nullable
as int,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}

/// @nodoc
mixin _$SessionAnswerRecord {

 String get questionId; int get position; bool get answered; bool? get correct;
/// Create a copy of SessionAnswerRecord
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SessionAnswerRecordCopyWith<SessionAnswerRecord> get copyWith => _$SessionAnswerRecordCopyWithImpl<SessionAnswerRecord>(this as SessionAnswerRecord, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SessionAnswerRecord&&(identical(other.questionId, questionId) || other.questionId == questionId)&&(identical(other.position, position) || other.position == position)&&(identical(other.answered, answered) || other.answered == answered)&&(identical(other.correct, correct) || other.correct == correct));
}


@override
int get hashCode => Object.hash(runtimeType,questionId,position,answered,correct);

@override
String toString() {
  return 'SessionAnswerRecord(questionId: $questionId, position: $position, answered: $answered, correct: $correct)';
}


}

/// @nodoc
abstract mixin class $SessionAnswerRecordCopyWith<$Res>  {
  factory $SessionAnswerRecordCopyWith(SessionAnswerRecord value, $Res Function(SessionAnswerRecord) _then) = _$SessionAnswerRecordCopyWithImpl;
@useResult
$Res call({
 String questionId, int position, bool answered, bool? correct
});




}
/// @nodoc
class _$SessionAnswerRecordCopyWithImpl<$Res>
    implements $SessionAnswerRecordCopyWith<$Res> {
  _$SessionAnswerRecordCopyWithImpl(this._self, this._then);

  final SessionAnswerRecord _self;
  final $Res Function(SessionAnswerRecord) _then;

/// Create a copy of SessionAnswerRecord
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? questionId = null,Object? position = null,Object? answered = null,Object? correct = freezed,}) {
  return _then(_self.copyWith(
questionId: null == questionId ? _self.questionId : questionId // ignore: cast_nullable_to_non_nullable
as String,position: null == position ? _self.position : position // ignore: cast_nullable_to_non_nullable
as int,answered: null == answered ? _self.answered : answered // ignore: cast_nullable_to_non_nullable
as bool,correct: freezed == correct ? _self.correct : correct // ignore: cast_nullable_to_non_nullable
as bool?,
  ));
}

}


/// Adds pattern-matching-related methods to [SessionAnswerRecord].
extension SessionAnswerRecordPatterns on SessionAnswerRecord {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SessionAnswerRecord value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SessionAnswerRecord() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SessionAnswerRecord value)  $default,){
final _that = this;
switch (_that) {
case _SessionAnswerRecord():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SessionAnswerRecord value)?  $default,){
final _that = this;
switch (_that) {
case _SessionAnswerRecord() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String questionId,  int position,  bool answered,  bool? correct)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SessionAnswerRecord() when $default != null:
return $default(_that.questionId,_that.position,_that.answered,_that.correct);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String questionId,  int position,  bool answered,  bool? correct)  $default,) {final _that = this;
switch (_that) {
case _SessionAnswerRecord():
return $default(_that.questionId,_that.position,_that.answered,_that.correct);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String questionId,  int position,  bool answered,  bool? correct)?  $default,) {final _that = this;
switch (_that) {
case _SessionAnswerRecord() when $default != null:
return $default(_that.questionId,_that.position,_that.answered,_that.correct);case _:
  return null;

}
}

}

/// @nodoc


class _SessionAnswerRecord implements SessionAnswerRecord {
  const _SessionAnswerRecord({required this.questionId, required this.position, required this.answered, this.correct});
  

@override final  String questionId;
@override final  int position;
@override final  bool answered;
@override final  bool? correct;

/// Create a copy of SessionAnswerRecord
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SessionAnswerRecordCopyWith<_SessionAnswerRecord> get copyWith => __$SessionAnswerRecordCopyWithImpl<_SessionAnswerRecord>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SessionAnswerRecord&&(identical(other.questionId, questionId) || other.questionId == questionId)&&(identical(other.position, position) || other.position == position)&&(identical(other.answered, answered) || other.answered == answered)&&(identical(other.correct, correct) || other.correct == correct));
}


@override
int get hashCode => Object.hash(runtimeType,questionId,position,answered,correct);

@override
String toString() {
  return 'SessionAnswerRecord(questionId: $questionId, position: $position, answered: $answered, correct: $correct)';
}


}

/// @nodoc
abstract mixin class _$SessionAnswerRecordCopyWith<$Res> implements $SessionAnswerRecordCopyWith<$Res> {
  factory _$SessionAnswerRecordCopyWith(_SessionAnswerRecord value, $Res Function(_SessionAnswerRecord) _then) = __$SessionAnswerRecordCopyWithImpl;
@override @useResult
$Res call({
 String questionId, int position, bool answered, bool? correct
});




}
/// @nodoc
class __$SessionAnswerRecordCopyWithImpl<$Res>
    implements _$SessionAnswerRecordCopyWith<$Res> {
  __$SessionAnswerRecordCopyWithImpl(this._self, this._then);

  final _SessionAnswerRecord _self;
  final $Res Function(_SessionAnswerRecord) _then;

/// Create a copy of SessionAnswerRecord
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? questionId = null,Object? position = null,Object? answered = null,Object? correct = freezed,}) {
  return _then(_SessionAnswerRecord(
questionId: null == questionId ? _self.questionId : questionId // ignore: cast_nullable_to_non_nullable
as String,position: null == position ? _self.position : position // ignore: cast_nullable_to_non_nullable
as int,answered: null == answered ? _self.answered : answered // ignore: cast_nullable_to_non_nullable
as bool,correct: freezed == correct ? _self.correct : correct // ignore: cast_nullable_to_non_nullable
as bool?,
  ));
}


}

/// @nodoc
mixin _$SessionDetail {

 String get id; String get mode; int get total; String get status; String get stoppedReason; int? get score; DateTime get startedAt; DateTime? get finishedAt; List<SessionAnswerRecord> get answers;
/// Create a copy of SessionDetail
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SessionDetailCopyWith<SessionDetail> get copyWith => _$SessionDetailCopyWithImpl<SessionDetail>(this as SessionDetail, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SessionDetail&&(identical(other.id, id) || other.id == id)&&(identical(other.mode, mode) || other.mode == mode)&&(identical(other.total, total) || other.total == total)&&(identical(other.status, status) || other.status == status)&&(identical(other.stoppedReason, stoppedReason) || other.stoppedReason == stoppedReason)&&(identical(other.score, score) || other.score == score)&&(identical(other.startedAt, startedAt) || other.startedAt == startedAt)&&(identical(other.finishedAt, finishedAt) || other.finishedAt == finishedAt)&&const DeepCollectionEquality().equals(other.answers, answers));
}


@override
int get hashCode => Object.hash(runtimeType,id,mode,total,status,stoppedReason,score,startedAt,finishedAt,const DeepCollectionEquality().hash(answers));

@override
String toString() {
  return 'SessionDetail(id: $id, mode: $mode, total: $total, status: $status, stoppedReason: $stoppedReason, score: $score, startedAt: $startedAt, finishedAt: $finishedAt, answers: $answers)';
}


}

/// @nodoc
abstract mixin class $SessionDetailCopyWith<$Res>  {
  factory $SessionDetailCopyWith(SessionDetail value, $Res Function(SessionDetail) _then) = _$SessionDetailCopyWithImpl;
@useResult
$Res call({
 String id, String mode, int total, String status, String stoppedReason, int? score, DateTime startedAt, DateTime? finishedAt, List<SessionAnswerRecord> answers
});




}
/// @nodoc
class _$SessionDetailCopyWithImpl<$Res>
    implements $SessionDetailCopyWith<$Res> {
  _$SessionDetailCopyWithImpl(this._self, this._then);

  final SessionDetail _self;
  final $Res Function(SessionDetail) _then;

/// Create a copy of SessionDetail
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? mode = null,Object? total = null,Object? status = null,Object? stoppedReason = null,Object? score = freezed,Object? startedAt = null,Object? finishedAt = freezed,Object? answers = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,mode: null == mode ? _self.mode : mode // ignore: cast_nullable_to_non_nullable
as String,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,stoppedReason: null == stoppedReason ? _self.stoppedReason : stoppedReason // ignore: cast_nullable_to_non_nullable
as String,score: freezed == score ? _self.score : score // ignore: cast_nullable_to_non_nullable
as int?,startedAt: null == startedAt ? _self.startedAt : startedAt // ignore: cast_nullable_to_non_nullable
as DateTime,finishedAt: freezed == finishedAt ? _self.finishedAt : finishedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,answers: null == answers ? _self.answers : answers // ignore: cast_nullable_to_non_nullable
as List<SessionAnswerRecord>,
  ));
}

}


/// Adds pattern-matching-related methods to [SessionDetail].
extension SessionDetailPatterns on SessionDetail {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SessionDetail value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SessionDetail() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SessionDetail value)  $default,){
final _that = this;
switch (_that) {
case _SessionDetail():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SessionDetail value)?  $default,){
final _that = this;
switch (_that) {
case _SessionDetail() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String mode,  int total,  String status,  String stoppedReason,  int? score,  DateTime startedAt,  DateTime? finishedAt,  List<SessionAnswerRecord> answers)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SessionDetail() when $default != null:
return $default(_that.id,_that.mode,_that.total,_that.status,_that.stoppedReason,_that.score,_that.startedAt,_that.finishedAt,_that.answers);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String mode,  int total,  String status,  String stoppedReason,  int? score,  DateTime startedAt,  DateTime? finishedAt,  List<SessionAnswerRecord> answers)  $default,) {final _that = this;
switch (_that) {
case _SessionDetail():
return $default(_that.id,_that.mode,_that.total,_that.status,_that.stoppedReason,_that.score,_that.startedAt,_that.finishedAt,_that.answers);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String mode,  int total,  String status,  String stoppedReason,  int? score,  DateTime startedAt,  DateTime? finishedAt,  List<SessionAnswerRecord> answers)?  $default,) {final _that = this;
switch (_that) {
case _SessionDetail() when $default != null:
return $default(_that.id,_that.mode,_that.total,_that.status,_that.stoppedReason,_that.score,_that.startedAt,_that.finishedAt,_that.answers);case _:
  return null;

}
}

}

/// @nodoc


class _SessionDetail implements SessionDetail {
  const _SessionDetail({required this.id, required this.mode, required this.total, required this.status, required this.stoppedReason, this.score, required this.startedAt, this.finishedAt, required final  List<SessionAnswerRecord> answers}): _answers = answers;
  

@override final  String id;
@override final  String mode;
@override final  int total;
@override final  String status;
@override final  String stoppedReason;
@override final  int? score;
@override final  DateTime startedAt;
@override final  DateTime? finishedAt;
 final  List<SessionAnswerRecord> _answers;
@override List<SessionAnswerRecord> get answers {
  if (_answers is EqualUnmodifiableListView) return _answers;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_answers);
}


/// Create a copy of SessionDetail
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SessionDetailCopyWith<_SessionDetail> get copyWith => __$SessionDetailCopyWithImpl<_SessionDetail>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SessionDetail&&(identical(other.id, id) || other.id == id)&&(identical(other.mode, mode) || other.mode == mode)&&(identical(other.total, total) || other.total == total)&&(identical(other.status, status) || other.status == status)&&(identical(other.stoppedReason, stoppedReason) || other.stoppedReason == stoppedReason)&&(identical(other.score, score) || other.score == score)&&(identical(other.startedAt, startedAt) || other.startedAt == startedAt)&&(identical(other.finishedAt, finishedAt) || other.finishedAt == finishedAt)&&const DeepCollectionEquality().equals(other._answers, _answers));
}


@override
int get hashCode => Object.hash(runtimeType,id,mode,total,status,stoppedReason,score,startedAt,finishedAt,const DeepCollectionEquality().hash(_answers));

@override
String toString() {
  return 'SessionDetail(id: $id, mode: $mode, total: $total, status: $status, stoppedReason: $stoppedReason, score: $score, startedAt: $startedAt, finishedAt: $finishedAt, answers: $answers)';
}


}

/// @nodoc
abstract mixin class _$SessionDetailCopyWith<$Res> implements $SessionDetailCopyWith<$Res> {
  factory _$SessionDetailCopyWith(_SessionDetail value, $Res Function(_SessionDetail) _then) = __$SessionDetailCopyWithImpl;
@override @useResult
$Res call({
 String id, String mode, int total, String status, String stoppedReason, int? score, DateTime startedAt, DateTime? finishedAt, List<SessionAnswerRecord> answers
});




}
/// @nodoc
class __$SessionDetailCopyWithImpl<$Res>
    implements _$SessionDetailCopyWith<$Res> {
  __$SessionDetailCopyWithImpl(this._self, this._then);

  final _SessionDetail _self;
  final $Res Function(_SessionDetail) _then;

/// Create a copy of SessionDetail
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? mode = null,Object? total = null,Object? status = null,Object? stoppedReason = null,Object? score = freezed,Object? startedAt = null,Object? finishedAt = freezed,Object? answers = null,}) {
  return _then(_SessionDetail(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,mode: null == mode ? _self.mode : mode // ignore: cast_nullable_to_non_nullable
as String,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,stoppedReason: null == stoppedReason ? _self.stoppedReason : stoppedReason // ignore: cast_nullable_to_non_nullable
as String,score: freezed == score ? _self.score : score // ignore: cast_nullable_to_non_nullable
as int?,startedAt: null == startedAt ? _self.startedAt : startedAt // ignore: cast_nullable_to_non_nullable
as DateTime,finishedAt: freezed == finishedAt ? _self.finishedAt : finishedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,answers: null == answers ? _self._answers : answers // ignore: cast_nullable_to_non_nullable
as List<SessionAnswerRecord>,
  ));
}


}

/// @nodoc
mixin _$VariantStatus {

 int get number; int get questionCount; bool get unlocked; int get bestCorrect; int get attempts; DateTime? get completedAt;
/// Create a copy of VariantStatus
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$VariantStatusCopyWith<VariantStatus> get copyWith => _$VariantStatusCopyWithImpl<VariantStatus>(this as VariantStatus, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is VariantStatus&&(identical(other.number, number) || other.number == number)&&(identical(other.questionCount, questionCount) || other.questionCount == questionCount)&&(identical(other.unlocked, unlocked) || other.unlocked == unlocked)&&(identical(other.bestCorrect, bestCorrect) || other.bestCorrect == bestCorrect)&&(identical(other.attempts, attempts) || other.attempts == attempts)&&(identical(other.completedAt, completedAt) || other.completedAt == completedAt));
}


@override
int get hashCode => Object.hash(runtimeType,number,questionCount,unlocked,bestCorrect,attempts,completedAt);

@override
String toString() {
  return 'VariantStatus(number: $number, questionCount: $questionCount, unlocked: $unlocked, bestCorrect: $bestCorrect, attempts: $attempts, completedAt: $completedAt)';
}


}

/// @nodoc
abstract mixin class $VariantStatusCopyWith<$Res>  {
  factory $VariantStatusCopyWith(VariantStatus value, $Res Function(VariantStatus) _then) = _$VariantStatusCopyWithImpl;
@useResult
$Res call({
 int number, int questionCount, bool unlocked, int bestCorrect, int attempts, DateTime? completedAt
});




}
/// @nodoc
class _$VariantStatusCopyWithImpl<$Res>
    implements $VariantStatusCopyWith<$Res> {
  _$VariantStatusCopyWithImpl(this._self, this._then);

  final VariantStatus _self;
  final $Res Function(VariantStatus) _then;

/// Create a copy of VariantStatus
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? number = null,Object? questionCount = null,Object? unlocked = null,Object? bestCorrect = null,Object? attempts = null,Object? completedAt = freezed,}) {
  return _then(_self.copyWith(
number: null == number ? _self.number : number // ignore: cast_nullable_to_non_nullable
as int,questionCount: null == questionCount ? _self.questionCount : questionCount // ignore: cast_nullable_to_non_nullable
as int,unlocked: null == unlocked ? _self.unlocked : unlocked // ignore: cast_nullable_to_non_nullable
as bool,bestCorrect: null == bestCorrect ? _self.bestCorrect : bestCorrect // ignore: cast_nullable_to_non_nullable
as int,attempts: null == attempts ? _self.attempts : attempts // ignore: cast_nullable_to_non_nullable
as int,completedAt: freezed == completedAt ? _self.completedAt : completedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}

}


/// Adds pattern-matching-related methods to [VariantStatus].
extension VariantStatusPatterns on VariantStatus {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _VariantStatus value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _VariantStatus() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _VariantStatus value)  $default,){
final _that = this;
switch (_that) {
case _VariantStatus():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _VariantStatus value)?  $default,){
final _that = this;
switch (_that) {
case _VariantStatus() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( int number,  int questionCount,  bool unlocked,  int bestCorrect,  int attempts,  DateTime? completedAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _VariantStatus() when $default != null:
return $default(_that.number,_that.questionCount,_that.unlocked,_that.bestCorrect,_that.attempts,_that.completedAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( int number,  int questionCount,  bool unlocked,  int bestCorrect,  int attempts,  DateTime? completedAt)  $default,) {final _that = this;
switch (_that) {
case _VariantStatus():
return $default(_that.number,_that.questionCount,_that.unlocked,_that.bestCorrect,_that.attempts,_that.completedAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( int number,  int questionCount,  bool unlocked,  int bestCorrect,  int attempts,  DateTime? completedAt)?  $default,) {final _that = this;
switch (_that) {
case _VariantStatus() when $default != null:
return $default(_that.number,_that.questionCount,_that.unlocked,_that.bestCorrect,_that.attempts,_that.completedAt);case _:
  return null;

}
}

}

/// @nodoc


class _VariantStatus implements VariantStatus {
  const _VariantStatus({required this.number, required this.questionCount, required this.unlocked, required this.bestCorrect, required this.attempts, this.completedAt});
  

@override final  int number;
@override final  int questionCount;
@override final  bool unlocked;
@override final  int bestCorrect;
@override final  int attempts;
@override final  DateTime? completedAt;

/// Create a copy of VariantStatus
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$VariantStatusCopyWith<_VariantStatus> get copyWith => __$VariantStatusCopyWithImpl<_VariantStatus>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _VariantStatus&&(identical(other.number, number) || other.number == number)&&(identical(other.questionCount, questionCount) || other.questionCount == questionCount)&&(identical(other.unlocked, unlocked) || other.unlocked == unlocked)&&(identical(other.bestCorrect, bestCorrect) || other.bestCorrect == bestCorrect)&&(identical(other.attempts, attempts) || other.attempts == attempts)&&(identical(other.completedAt, completedAt) || other.completedAt == completedAt));
}


@override
int get hashCode => Object.hash(runtimeType,number,questionCount,unlocked,bestCorrect,attempts,completedAt);

@override
String toString() {
  return 'VariantStatus(number: $number, questionCount: $questionCount, unlocked: $unlocked, bestCorrect: $bestCorrect, attempts: $attempts, completedAt: $completedAt)';
}


}

/// @nodoc
abstract mixin class _$VariantStatusCopyWith<$Res> implements $VariantStatusCopyWith<$Res> {
  factory _$VariantStatusCopyWith(_VariantStatus value, $Res Function(_VariantStatus) _then) = __$VariantStatusCopyWithImpl;
@override @useResult
$Res call({
 int number, int questionCount, bool unlocked, int bestCorrect, int attempts, DateTime? completedAt
});




}
/// @nodoc
class __$VariantStatusCopyWithImpl<$Res>
    implements _$VariantStatusCopyWith<$Res> {
  __$VariantStatusCopyWithImpl(this._self, this._then);

  final _VariantStatus _self;
  final $Res Function(_VariantStatus) _then;

/// Create a copy of VariantStatus
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? number = null,Object? questionCount = null,Object? unlocked = null,Object? bestCorrect = null,Object? attempts = null,Object? completedAt = freezed,}) {
  return _then(_VariantStatus(
number: null == number ? _self.number : number // ignore: cast_nullable_to_non_nullable
as int,questionCount: null == questionCount ? _self.questionCount : questionCount // ignore: cast_nullable_to_non_nullable
as int,unlocked: null == unlocked ? _self.unlocked : unlocked // ignore: cast_nullable_to_non_nullable
as bool,bestCorrect: null == bestCorrect ? _self.bestCorrect : bestCorrect // ignore: cast_nullable_to_non_nullable
as int,attempts: null == attempts ? _self.attempts : attempts // ignore: cast_nullable_to_non_nullable
as int,completedAt: freezed == completedAt ? _self.completedAt : completedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}


}

// dart format on
