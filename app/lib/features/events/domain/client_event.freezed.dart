// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'client_event.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$ClientEvent {

 String get name; Map<String, dynamic>? get props; DateTime? get ts;
/// Create a copy of ClientEvent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ClientEventCopyWith<ClientEvent> get copyWith => _$ClientEventCopyWithImpl<ClientEvent>(this as ClientEvent, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ClientEvent&&(identical(other.name, name) || other.name == name)&&const DeepCollectionEquality().equals(other.props, props)&&(identical(other.ts, ts) || other.ts == ts));
}


@override
int get hashCode => Object.hash(runtimeType,name,const DeepCollectionEquality().hash(props),ts);

@override
String toString() {
  return 'ClientEvent(name: $name, props: $props, ts: $ts)';
}


}

/// @nodoc
abstract mixin class $ClientEventCopyWith<$Res>  {
  factory $ClientEventCopyWith(ClientEvent value, $Res Function(ClientEvent) _then) = _$ClientEventCopyWithImpl;
@useResult
$Res call({
 String name, Map<String, dynamic>? props, DateTime? ts
});




}
/// @nodoc
class _$ClientEventCopyWithImpl<$Res>
    implements $ClientEventCopyWith<$Res> {
  _$ClientEventCopyWithImpl(this._self, this._then);

  final ClientEvent _self;
  final $Res Function(ClientEvent) _then;

/// Create a copy of ClientEvent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? name = null,Object? props = freezed,Object? ts = freezed,}) {
  return _then(_self.copyWith(
name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,props: freezed == props ? _self.props : props // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,ts: freezed == ts ? _self.ts : ts // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}

}


/// Adds pattern-matching-related methods to [ClientEvent].
extension ClientEventPatterns on ClientEvent {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _ClientEvent value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _ClientEvent() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _ClientEvent value)  $default,){
final _that = this;
switch (_that) {
case _ClientEvent():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _ClientEvent value)?  $default,){
final _that = this;
switch (_that) {
case _ClientEvent() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String name,  Map<String, dynamic>? props,  DateTime? ts)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _ClientEvent() when $default != null:
return $default(_that.name,_that.props,_that.ts);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String name,  Map<String, dynamic>? props,  DateTime? ts)  $default,) {final _that = this;
switch (_that) {
case _ClientEvent():
return $default(_that.name,_that.props,_that.ts);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String name,  Map<String, dynamic>? props,  DateTime? ts)?  $default,) {final _that = this;
switch (_that) {
case _ClientEvent() when $default != null:
return $default(_that.name,_that.props,_that.ts);case _:
  return null;

}
}

}

/// @nodoc


class _ClientEvent implements ClientEvent {
  const _ClientEvent({required this.name, final  Map<String, dynamic>? props, this.ts}): _props = props;
  

@override final  String name;
 final  Map<String, dynamic>? _props;
@override Map<String, dynamic>? get props {
  final value = _props;
  if (value == null) return null;
  if (_props is EqualUnmodifiableMapView) return _props;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableMapView(value);
}

@override final  DateTime? ts;

/// Create a copy of ClientEvent
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ClientEventCopyWith<_ClientEvent> get copyWith => __$ClientEventCopyWithImpl<_ClientEvent>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _ClientEvent&&(identical(other.name, name) || other.name == name)&&const DeepCollectionEquality().equals(other._props, _props)&&(identical(other.ts, ts) || other.ts == ts));
}


@override
int get hashCode => Object.hash(runtimeType,name,const DeepCollectionEquality().hash(_props),ts);

@override
String toString() {
  return 'ClientEvent(name: $name, props: $props, ts: $ts)';
}


}

/// @nodoc
abstract mixin class _$ClientEventCopyWith<$Res> implements $ClientEventCopyWith<$Res> {
  factory _$ClientEventCopyWith(_ClientEvent value, $Res Function(_ClientEvent) _then) = __$ClientEventCopyWithImpl;
@override @useResult
$Res call({
 String name, Map<String, dynamic>? props, DateTime? ts
});




}
/// @nodoc
class __$ClientEventCopyWithImpl<$Res>
    implements _$ClientEventCopyWith<$Res> {
  __$ClientEventCopyWithImpl(this._self, this._then);

  final _ClientEvent _self;
  final $Res Function(_ClientEvent) _then;

/// Create a copy of ClientEvent
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? name = null,Object? props = freezed,Object? ts = freezed,}) {
  return _then(_ClientEvent(
name: null == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String,props: freezed == props ? _self._props : props // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,ts: freezed == ts ? _self.ts : ts // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}


}

// dart format on
