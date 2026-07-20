// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'session_state.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$SessionUiState {





@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SessionUiState);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'SessionUiState()';
}


}

/// @nodoc
class $SessionUiStateCopyWith<$Res>  {
$SessionUiStateCopyWith(SessionUiState _, $Res Function(SessionUiState) __);
}


/// Adds pattern-matching-related methods to [SessionUiState].
extension SessionUiStatePatterns on SessionUiState {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>({TResult Function( SessionLoading value)?  loading,TResult Function( SessionActive value)?  active,TResult Function( SessionStopped value)?  stopped,TResult Function( SessionFinished value)?  finished,TResult Function( SessionError value)?  error,required TResult orElse(),}){
final _that = this;
switch (_that) {
case SessionLoading() when loading != null:
return loading(_that);case SessionActive() when active != null:
return active(_that);case SessionStopped() when stopped != null:
return stopped(_that);case SessionFinished() when finished != null:
return finished(_that);case SessionError() when error != null:
return error(_that);case _:
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

@optionalTypeArgs TResult map<TResult extends Object?>({required TResult Function( SessionLoading value)  loading,required TResult Function( SessionActive value)  active,required TResult Function( SessionStopped value)  stopped,required TResult Function( SessionFinished value)  finished,required TResult Function( SessionError value)  error,}){
final _that = this;
switch (_that) {
case SessionLoading():
return loading(_that);case SessionActive():
return active(_that);case SessionStopped():
return stopped(_that);case SessionFinished():
return finished(_that);case SessionError():
return error(_that);}
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>({TResult? Function( SessionLoading value)?  loading,TResult? Function( SessionActive value)?  active,TResult? Function( SessionStopped value)?  stopped,TResult? Function( SessionFinished value)?  finished,TResult? Function( SessionError value)?  error,}){
final _that = this;
switch (_that) {
case SessionLoading() when loading != null:
return loading(_that);case SessionActive() when active != null:
return active(_that);case SessionStopped() when stopped != null:
return stopped(_that);case SessionFinished() when finished != null:
return finished(_that);case SessionError() when error != null:
return error(_that);case _:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>({TResult Function()?  loading,TResult Function( SessionSummary summary,  int currentIndex,  Map<String, AnswerResult> answered,  Duration? remaining,  String? pendingRetryQuestionId,  String? pendingRetryAnswerId)?  active,TResult Function( SessionSummary summary,  String stopReason)?  stopped,TResult Function( SessionResult result)?  finished,TResult Function( Failure failure)?  error,required TResult orElse(),}) {final _that = this;
switch (_that) {
case SessionLoading() when loading != null:
return loading();case SessionActive() when active != null:
return active(_that.summary,_that.currentIndex,_that.answered,_that.remaining,_that.pendingRetryQuestionId,_that.pendingRetryAnswerId);case SessionStopped() when stopped != null:
return stopped(_that.summary,_that.stopReason);case SessionFinished() when finished != null:
return finished(_that.result);case SessionError() when error != null:
return error(_that.failure);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>({required TResult Function()  loading,required TResult Function( SessionSummary summary,  int currentIndex,  Map<String, AnswerResult> answered,  Duration? remaining,  String? pendingRetryQuestionId,  String? pendingRetryAnswerId)  active,required TResult Function( SessionSummary summary,  String stopReason)  stopped,required TResult Function( SessionResult result)  finished,required TResult Function( Failure failure)  error,}) {final _that = this;
switch (_that) {
case SessionLoading():
return loading();case SessionActive():
return active(_that.summary,_that.currentIndex,_that.answered,_that.remaining,_that.pendingRetryQuestionId,_that.pendingRetryAnswerId);case SessionStopped():
return stopped(_that.summary,_that.stopReason);case SessionFinished():
return finished(_that.result);case SessionError():
return error(_that.failure);}
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>({TResult? Function()?  loading,TResult? Function( SessionSummary summary,  int currentIndex,  Map<String, AnswerResult> answered,  Duration? remaining,  String? pendingRetryQuestionId,  String? pendingRetryAnswerId)?  active,TResult? Function( SessionSummary summary,  String stopReason)?  stopped,TResult? Function( SessionResult result)?  finished,TResult? Function( Failure failure)?  error,}) {final _that = this;
switch (_that) {
case SessionLoading() when loading != null:
return loading();case SessionActive() when active != null:
return active(_that.summary,_that.currentIndex,_that.answered,_that.remaining,_that.pendingRetryQuestionId,_that.pendingRetryAnswerId);case SessionStopped() when stopped != null:
return stopped(_that.summary,_that.stopReason);case SessionFinished() when finished != null:
return finished(_that.result);case SessionError() when error != null:
return error(_that.failure);case _:
  return null;

}
}

}

/// @nodoc


class SessionLoading implements SessionUiState {
  const SessionLoading();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SessionLoading);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'SessionUiState.loading()';
}


}




/// @nodoc


class SessionActive implements SessionUiState {
  const SessionActive({required this.summary, required this.currentIndex, required final  Map<String, AnswerResult> answered, this.remaining, this.pendingRetryQuestionId, this.pendingRetryAnswerId}): _answered = answered;
  

 final  SessionSummary summary;
 final  int currentIndex;
 final  Map<String, AnswerResult> _answered;
 Map<String, AnswerResult> get answered {
  if (_answered is EqualUnmodifiableMapView) return _answered;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableMapView(_answered);
}

 final  Duration? remaining;
 final  String? pendingRetryQuestionId;
 final  String? pendingRetryAnswerId;

/// Create a copy of SessionUiState
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SessionActiveCopyWith<SessionActive> get copyWith => _$SessionActiveCopyWithImpl<SessionActive>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SessionActive&&(identical(other.summary, summary) || other.summary == summary)&&(identical(other.currentIndex, currentIndex) || other.currentIndex == currentIndex)&&const DeepCollectionEquality().equals(other._answered, _answered)&&(identical(other.remaining, remaining) || other.remaining == remaining)&&(identical(other.pendingRetryQuestionId, pendingRetryQuestionId) || other.pendingRetryQuestionId == pendingRetryQuestionId)&&(identical(other.pendingRetryAnswerId, pendingRetryAnswerId) || other.pendingRetryAnswerId == pendingRetryAnswerId));
}


@override
int get hashCode => Object.hash(runtimeType,summary,currentIndex,const DeepCollectionEquality().hash(_answered),remaining,pendingRetryQuestionId,pendingRetryAnswerId);

@override
String toString() {
  return 'SessionUiState.active(summary: $summary, currentIndex: $currentIndex, answered: $answered, remaining: $remaining, pendingRetryQuestionId: $pendingRetryQuestionId, pendingRetryAnswerId: $pendingRetryAnswerId)';
}


}

/// @nodoc
abstract mixin class $SessionActiveCopyWith<$Res> implements $SessionUiStateCopyWith<$Res> {
  factory $SessionActiveCopyWith(SessionActive value, $Res Function(SessionActive) _then) = _$SessionActiveCopyWithImpl;
@useResult
$Res call({
 SessionSummary summary, int currentIndex, Map<String, AnswerResult> answered, Duration? remaining, String? pendingRetryQuestionId, String? pendingRetryAnswerId
});


$SessionSummaryCopyWith<$Res> get summary;

}
/// @nodoc
class _$SessionActiveCopyWithImpl<$Res>
    implements $SessionActiveCopyWith<$Res> {
  _$SessionActiveCopyWithImpl(this._self, this._then);

  final SessionActive _self;
  final $Res Function(SessionActive) _then;

/// Create a copy of SessionUiState
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? summary = null,Object? currentIndex = null,Object? answered = null,Object? remaining = freezed,Object? pendingRetryQuestionId = freezed,Object? pendingRetryAnswerId = freezed,}) {
  return _then(SessionActive(
summary: null == summary ? _self.summary : summary // ignore: cast_nullable_to_non_nullable
as SessionSummary,currentIndex: null == currentIndex ? _self.currentIndex : currentIndex // ignore: cast_nullable_to_non_nullable
as int,answered: null == answered ? _self._answered : answered // ignore: cast_nullable_to_non_nullable
as Map<String, AnswerResult>,remaining: freezed == remaining ? _self.remaining : remaining // ignore: cast_nullable_to_non_nullable
as Duration?,pendingRetryQuestionId: freezed == pendingRetryQuestionId ? _self.pendingRetryQuestionId : pendingRetryQuestionId // ignore: cast_nullable_to_non_nullable
as String?,pendingRetryAnswerId: freezed == pendingRetryAnswerId ? _self.pendingRetryAnswerId : pendingRetryAnswerId // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

/// Create a copy of SessionUiState
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SessionSummaryCopyWith<$Res> get summary {
  
  return $SessionSummaryCopyWith<$Res>(_self.summary, (value) {
    return _then(_self.copyWith(summary: value));
  });
}
}

/// @nodoc


class SessionStopped implements SessionUiState {
  const SessionStopped({required this.summary, required this.stopReason});
  

 final  SessionSummary summary;
 final  String stopReason;

/// Create a copy of SessionUiState
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SessionStoppedCopyWith<SessionStopped> get copyWith => _$SessionStoppedCopyWithImpl<SessionStopped>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SessionStopped&&(identical(other.summary, summary) || other.summary == summary)&&(identical(other.stopReason, stopReason) || other.stopReason == stopReason));
}


@override
int get hashCode => Object.hash(runtimeType,summary,stopReason);

@override
String toString() {
  return 'SessionUiState.stopped(summary: $summary, stopReason: $stopReason)';
}


}

/// @nodoc
abstract mixin class $SessionStoppedCopyWith<$Res> implements $SessionUiStateCopyWith<$Res> {
  factory $SessionStoppedCopyWith(SessionStopped value, $Res Function(SessionStopped) _then) = _$SessionStoppedCopyWithImpl;
@useResult
$Res call({
 SessionSummary summary, String stopReason
});


$SessionSummaryCopyWith<$Res> get summary;

}
/// @nodoc
class _$SessionStoppedCopyWithImpl<$Res>
    implements $SessionStoppedCopyWith<$Res> {
  _$SessionStoppedCopyWithImpl(this._self, this._then);

  final SessionStopped _self;
  final $Res Function(SessionStopped) _then;

/// Create a copy of SessionUiState
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? summary = null,Object? stopReason = null,}) {
  return _then(SessionStopped(
summary: null == summary ? _self.summary : summary // ignore: cast_nullable_to_non_nullable
as SessionSummary,stopReason: null == stopReason ? _self.stopReason : stopReason // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

/// Create a copy of SessionUiState
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SessionSummaryCopyWith<$Res> get summary {
  
  return $SessionSummaryCopyWith<$Res>(_self.summary, (value) {
    return _then(_self.copyWith(summary: value));
  });
}
}

/// @nodoc


class SessionFinished implements SessionUiState {
  const SessionFinished({required this.result});
  

 final  SessionResult result;

/// Create a copy of SessionUiState
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SessionFinishedCopyWith<SessionFinished> get copyWith => _$SessionFinishedCopyWithImpl<SessionFinished>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SessionFinished&&(identical(other.result, result) || other.result == result));
}


@override
int get hashCode => Object.hash(runtimeType,result);

@override
String toString() {
  return 'SessionUiState.finished(result: $result)';
}


}

/// @nodoc
abstract mixin class $SessionFinishedCopyWith<$Res> implements $SessionUiStateCopyWith<$Res> {
  factory $SessionFinishedCopyWith(SessionFinished value, $Res Function(SessionFinished) _then) = _$SessionFinishedCopyWithImpl;
@useResult
$Res call({
 SessionResult result
});


$SessionResultCopyWith<$Res> get result;

}
/// @nodoc
class _$SessionFinishedCopyWithImpl<$Res>
    implements $SessionFinishedCopyWith<$Res> {
  _$SessionFinishedCopyWithImpl(this._self, this._then);

  final SessionFinished _self;
  final $Res Function(SessionFinished) _then;

/// Create a copy of SessionUiState
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? result = null,}) {
  return _then(SessionFinished(
result: null == result ? _self.result : result // ignore: cast_nullable_to_non_nullable
as SessionResult,
  ));
}

/// Create a copy of SessionUiState
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$SessionResultCopyWith<$Res> get result {
  
  return $SessionResultCopyWith<$Res>(_self.result, (value) {
    return _then(_self.copyWith(result: value));
  });
}
}

/// @nodoc


class SessionError implements SessionUiState {
  const SessionError(this.failure);
  

 final  Failure failure;

/// Create a copy of SessionUiState
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SessionErrorCopyWith<SessionError> get copyWith => _$SessionErrorCopyWithImpl<SessionError>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SessionError&&(identical(other.failure, failure) || other.failure == failure));
}


@override
int get hashCode => Object.hash(runtimeType,failure);

@override
String toString() {
  return 'SessionUiState.error(failure: $failure)';
}


}

/// @nodoc
abstract mixin class $SessionErrorCopyWith<$Res> implements $SessionUiStateCopyWith<$Res> {
  factory $SessionErrorCopyWith(SessionError value, $Res Function(SessionError) _then) = _$SessionErrorCopyWithImpl;
@useResult
$Res call({
 Failure failure
});


$FailureCopyWith<$Res> get failure;

}
/// @nodoc
class _$SessionErrorCopyWithImpl<$Res>
    implements $SessionErrorCopyWith<$Res> {
  _$SessionErrorCopyWithImpl(this._self, this._then);

  final SessionError _self;
  final $Res Function(SessionError) _then;

/// Create a copy of SessionUiState
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? failure = null,}) {
  return _then(SessionError(
null == failure ? _self.failure : failure // ignore: cast_nullable_to_non_nullable
as Failure,
  ));
}

/// Create a copy of SessionUiState
/// with the given fields replaced by the non-null parameter values.
@override
@pragma('vm:prefer-inline')
$FailureCopyWith<$Res> get failure {
  
  return $FailureCopyWith<$Res>(_self.failure, (value) {
    return _then(_self.copyWith(failure: value));
  });
}
}

// dart format on
