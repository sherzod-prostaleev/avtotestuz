// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'auth_state.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$AuthState {





@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AuthState);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'AuthState()';
}


}

/// @nodoc
class $AuthStateCopyWith<$Res>  {
$AuthStateCopyWith(AuthState _, $Res Function(AuthState) __);
}


/// Adds pattern-matching-related methods to [AuthState].
extension AuthStatePatterns on AuthState {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>({TResult Function( AuthUnknown value)?  unknown,TResult Function( AuthUnauthenticated value)?  unauthenticated,TResult Function( AuthOtpRequested value)?  otpRequested,TResult Function( AuthAuthenticated value)?  authenticated,TResult Function( AuthError value)?  error,required TResult orElse(),}){
final _that = this;
switch (_that) {
case AuthUnknown() when unknown != null:
return unknown(_that);case AuthUnauthenticated() when unauthenticated != null:
return unauthenticated(_that);case AuthOtpRequested() when otpRequested != null:
return otpRequested(_that);case AuthAuthenticated() when authenticated != null:
return authenticated(_that);case AuthError() when error != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>({required TResult Function( AuthUnknown value)  unknown,required TResult Function( AuthUnauthenticated value)  unauthenticated,required TResult Function( AuthOtpRequested value)  otpRequested,required TResult Function( AuthAuthenticated value)  authenticated,required TResult Function( AuthError value)  error,}){
final _that = this;
switch (_that) {
case AuthUnknown():
return unknown(_that);case AuthUnauthenticated():
return unauthenticated(_that);case AuthOtpRequested():
return otpRequested(_that);case AuthAuthenticated():
return authenticated(_that);case AuthError():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>({TResult? Function( AuthUnknown value)?  unknown,TResult? Function( AuthUnauthenticated value)?  unauthenticated,TResult? Function( AuthOtpRequested value)?  otpRequested,TResult? Function( AuthAuthenticated value)?  authenticated,TResult? Function( AuthError value)?  error,}){
final _that = this;
switch (_that) {
case AuthUnknown() when unknown != null:
return unknown(_that);case AuthUnauthenticated() when unauthenticated != null:
return unauthenticated(_that);case AuthOtpRequested() when otpRequested != null:
return otpRequested(_that);case AuthAuthenticated() when authenticated != null:
return authenticated(_that);case AuthError() when error != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>({TResult Function()?  unknown,TResult Function()?  unauthenticated,TResult Function( String phone,  String? debugCode)?  otpRequested,TResult Function()?  authenticated,TResult Function( Failure failure)?  error,required TResult orElse(),}) {final _that = this;
switch (_that) {
case AuthUnknown() when unknown != null:
return unknown();case AuthUnauthenticated() when unauthenticated != null:
return unauthenticated();case AuthOtpRequested() when otpRequested != null:
return otpRequested(_that.phone,_that.debugCode);case AuthAuthenticated() when authenticated != null:
return authenticated();case AuthError() when error != null:
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

@optionalTypeArgs TResult when<TResult extends Object?>({required TResult Function()  unknown,required TResult Function()  unauthenticated,required TResult Function( String phone,  String? debugCode)  otpRequested,required TResult Function()  authenticated,required TResult Function( Failure failure)  error,}) {final _that = this;
switch (_that) {
case AuthUnknown():
return unknown();case AuthUnauthenticated():
return unauthenticated();case AuthOtpRequested():
return otpRequested(_that.phone,_that.debugCode);case AuthAuthenticated():
return authenticated();case AuthError():
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>({TResult? Function()?  unknown,TResult? Function()?  unauthenticated,TResult? Function( String phone,  String? debugCode)?  otpRequested,TResult? Function()?  authenticated,TResult? Function( Failure failure)?  error,}) {final _that = this;
switch (_that) {
case AuthUnknown() when unknown != null:
return unknown();case AuthUnauthenticated() when unauthenticated != null:
return unauthenticated();case AuthOtpRequested() when otpRequested != null:
return otpRequested(_that.phone,_that.debugCode);case AuthAuthenticated() when authenticated != null:
return authenticated();case AuthError() when error != null:
return error(_that.failure);case _:
  return null;

}
}

}

/// @nodoc


class AuthUnknown implements AuthState {
  const AuthUnknown();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AuthUnknown);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'AuthState.unknown()';
}


}




/// @nodoc


class AuthUnauthenticated implements AuthState {
  const AuthUnauthenticated();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AuthUnauthenticated);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'AuthState.unauthenticated()';
}


}




/// @nodoc


class AuthOtpRequested implements AuthState {
  const AuthOtpRequested({required this.phone, this.debugCode});
  

 final  String phone;
 final  String? debugCode;

/// Create a copy of AuthState
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AuthOtpRequestedCopyWith<AuthOtpRequested> get copyWith => _$AuthOtpRequestedCopyWithImpl<AuthOtpRequested>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AuthOtpRequested&&(identical(other.phone, phone) || other.phone == phone)&&(identical(other.debugCode, debugCode) || other.debugCode == debugCode));
}


@override
int get hashCode => Object.hash(runtimeType,phone,debugCode);

@override
String toString() {
  return 'AuthState.otpRequested(phone: $phone, debugCode: $debugCode)';
}


}

/// @nodoc
abstract mixin class $AuthOtpRequestedCopyWith<$Res> implements $AuthStateCopyWith<$Res> {
  factory $AuthOtpRequestedCopyWith(AuthOtpRequested value, $Res Function(AuthOtpRequested) _then) = _$AuthOtpRequestedCopyWithImpl;
@useResult
$Res call({
 String phone, String? debugCode
});




}
/// @nodoc
class _$AuthOtpRequestedCopyWithImpl<$Res>
    implements $AuthOtpRequestedCopyWith<$Res> {
  _$AuthOtpRequestedCopyWithImpl(this._self, this._then);

  final AuthOtpRequested _self;
  final $Res Function(AuthOtpRequested) _then;

/// Create a copy of AuthState
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? phone = null,Object? debugCode = freezed,}) {
  return _then(AuthOtpRequested(
phone: null == phone ? _self.phone : phone // ignore: cast_nullable_to_non_nullable
as String,debugCode: freezed == debugCode ? _self.debugCode : debugCode // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}

/// @nodoc


class AuthAuthenticated implements AuthState {
  const AuthAuthenticated();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AuthAuthenticated);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'AuthState.authenticated()';
}


}




/// @nodoc


class AuthError implements AuthState {
  const AuthError(this.failure);
  

 final  Failure failure;

/// Create a copy of AuthState
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AuthErrorCopyWith<AuthError> get copyWith => _$AuthErrorCopyWithImpl<AuthError>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AuthError&&(identical(other.failure, failure) || other.failure == failure));
}


@override
int get hashCode => Object.hash(runtimeType,failure);

@override
String toString() {
  return 'AuthState.error(failure: $failure)';
}


}

/// @nodoc
abstract mixin class $AuthErrorCopyWith<$Res> implements $AuthStateCopyWith<$Res> {
  factory $AuthErrorCopyWith(AuthError value, $Res Function(AuthError) _then) = _$AuthErrorCopyWithImpl;
@useResult
$Res call({
 Failure failure
});


$FailureCopyWith<$Res> get failure;

}
/// @nodoc
class _$AuthErrorCopyWithImpl<$Res>
    implements $AuthErrorCopyWith<$Res> {
  _$AuthErrorCopyWithImpl(this._self, this._then);

  final AuthError _self;
  final $Res Function(AuthError) _then;

/// Create a copy of AuthState
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? failure = null,}) {
  return _then(AuthError(
null == failure ? _self.failure : failure // ignore: cast_nullable_to_non_nullable
as Failure,
  ));
}

/// Create a copy of AuthState
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
