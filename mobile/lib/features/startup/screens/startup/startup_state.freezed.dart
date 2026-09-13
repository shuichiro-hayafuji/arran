// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'startup_state.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$StartupState {

 AsyncValue<StartupDecision?> get decision; String get baseUrl;
/// Create a copy of StartupState
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$StartupStateCopyWith<StartupState> get copyWith => _$StartupStateCopyWithImpl<StartupState>(this as StartupState, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is StartupState&&(identical(other.decision, decision) || other.decision == decision)&&(identical(other.baseUrl, baseUrl) || other.baseUrl == baseUrl));
}


@override
int get hashCode => Object.hash(runtimeType,decision,baseUrl);

@override
String toString() {
  return 'StartupState(decision: $decision, baseUrl: $baseUrl)';
}


}

/// @nodoc
abstract mixin class $StartupStateCopyWith<$Res>  {
  factory $StartupStateCopyWith(StartupState value, $Res Function(StartupState) _then) = _$StartupStateCopyWithImpl;
@useResult
$Res call({
 AsyncValue<StartupDecision?> decision, String baseUrl
});




}
/// @nodoc
class _$StartupStateCopyWithImpl<$Res>
    implements $StartupStateCopyWith<$Res> {
  _$StartupStateCopyWithImpl(this._self, this._then);

  final StartupState _self;
  final $Res Function(StartupState) _then;

/// Create a copy of StartupState
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? decision = null,Object? baseUrl = null,}) {
  return _then(_self.copyWith(
decision: null == decision ? _self.decision : decision // ignore: cast_nullable_to_non_nullable
as AsyncValue<StartupDecision?>,baseUrl: null == baseUrl ? _self.baseUrl : baseUrl // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [StartupState].
extension StartupStatePatterns on StartupState {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _StartupState value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _StartupState() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _StartupState value)  $default,){
final _that = this;
switch (_that) {
case _StartupState():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _StartupState value)?  $default,){
final _that = this;
switch (_that) {
case _StartupState() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( AsyncValue<StartupDecision?> decision,  String baseUrl)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _StartupState() when $default != null:
return $default(_that.decision,_that.baseUrl);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( AsyncValue<StartupDecision?> decision,  String baseUrl)  $default,) {final _that = this;
switch (_that) {
case _StartupState():
return $default(_that.decision,_that.baseUrl);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( AsyncValue<StartupDecision?> decision,  String baseUrl)?  $default,) {final _that = this;
switch (_that) {
case _StartupState() when $default != null:
return $default(_that.decision,_that.baseUrl);case _:
  return null;

}
}

}

/// @nodoc


class _StartupState implements StartupState {
  const _StartupState({required this.decision, required this.baseUrl});
  

@override final  AsyncValue<StartupDecision?> decision;
@override final  String baseUrl;

/// Create a copy of StartupState
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$StartupStateCopyWith<_StartupState> get copyWith => __$StartupStateCopyWithImpl<_StartupState>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _StartupState&&(identical(other.decision, decision) || other.decision == decision)&&(identical(other.baseUrl, baseUrl) || other.baseUrl == baseUrl));
}


@override
int get hashCode => Object.hash(runtimeType,decision,baseUrl);

@override
String toString() {
  return 'StartupState(decision: $decision, baseUrl: $baseUrl)';
}


}

/// @nodoc
abstract mixin class _$StartupStateCopyWith<$Res> implements $StartupStateCopyWith<$Res> {
  factory _$StartupStateCopyWith(_StartupState value, $Res Function(_StartupState) _then) = __$StartupStateCopyWithImpl;
@override @useResult
$Res call({
 AsyncValue<StartupDecision?> decision, String baseUrl
});




}
/// @nodoc
class __$StartupStateCopyWithImpl<$Res>
    implements _$StartupStateCopyWith<$Res> {
  __$StartupStateCopyWithImpl(this._self, this._then);

  final _StartupState _self;
  final $Res Function(_StartupState) _then;

/// Create a copy of StartupState
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? decision = null,Object? baseUrl = null,}) {
  return _then(_StartupState(
decision: null == decision ? _self.decision : decision // ignore: cast_nullable_to_non_nullable
as AsyncValue<StartupDecision?>,baseUrl: null == baseUrl ? _self.baseUrl : baseUrl // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

// dart format on
