// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'result_state.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$ResultState {

 AsyncValue<Consultation?> get consultation; bool get saving; bool get saved; Object? get actionError;
/// Create a copy of ResultState
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ResultStateCopyWith<ResultState> get copyWith => _$ResultStateCopyWithImpl<ResultState>(this as ResultState, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ResultState&&(identical(other.consultation, consultation) || other.consultation == consultation)&&(identical(other.saving, saving) || other.saving == saving)&&(identical(other.saved, saved) || other.saved == saved)&&const DeepCollectionEquality().equals(other.actionError, actionError));
}


@override
int get hashCode => Object.hash(runtimeType,consultation,saving,saved,const DeepCollectionEquality().hash(actionError));

@override
String toString() {
  return 'ResultState(consultation: $consultation, saving: $saving, saved: $saved, actionError: $actionError)';
}


}

/// @nodoc
abstract mixin class $ResultStateCopyWith<$Res>  {
  factory $ResultStateCopyWith(ResultState value, $Res Function(ResultState) _then) = _$ResultStateCopyWithImpl;
@useResult
$Res call({
 AsyncValue<Consultation?> consultation, bool saving, bool saved, Object? actionError
});




}
/// @nodoc
class _$ResultStateCopyWithImpl<$Res>
    implements $ResultStateCopyWith<$Res> {
  _$ResultStateCopyWithImpl(this._self, this._then);

  final ResultState _self;
  final $Res Function(ResultState) _then;

/// Create a copy of ResultState
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? consultation = null,Object? saving = null,Object? saved = null,Object? actionError = freezed,}) {
  return _then(_self.copyWith(
consultation: null == consultation ? _self.consultation : consultation // ignore: cast_nullable_to_non_nullable
as AsyncValue<Consultation?>,saving: null == saving ? _self.saving : saving // ignore: cast_nullable_to_non_nullable
as bool,saved: null == saved ? _self.saved : saved // ignore: cast_nullable_to_non_nullable
as bool,actionError: freezed == actionError ? _self.actionError : actionError ,
  ));
}

}


/// Adds pattern-matching-related methods to [ResultState].
extension ResultStatePatterns on ResultState {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _ResultState value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _ResultState() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _ResultState value)  $default,){
final _that = this;
switch (_that) {
case _ResultState():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _ResultState value)?  $default,){
final _that = this;
switch (_that) {
case _ResultState() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( AsyncValue<Consultation?> consultation,  bool saving,  bool saved,  Object? actionError)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _ResultState() when $default != null:
return $default(_that.consultation,_that.saving,_that.saved,_that.actionError);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( AsyncValue<Consultation?> consultation,  bool saving,  bool saved,  Object? actionError)  $default,) {final _that = this;
switch (_that) {
case _ResultState():
return $default(_that.consultation,_that.saving,_that.saved,_that.actionError);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( AsyncValue<Consultation?> consultation,  bool saving,  bool saved,  Object? actionError)?  $default,) {final _that = this;
switch (_that) {
case _ResultState() when $default != null:
return $default(_that.consultation,_that.saving,_that.saved,_that.actionError);case _:
  return null;

}
}

}

/// @nodoc


class _ResultState implements ResultState {
  const _ResultState({required this.consultation, this.saving = false, this.saved = false, this.actionError});
  

@override final  AsyncValue<Consultation?> consultation;
@override@JsonKey() final  bool saving;
@override@JsonKey() final  bool saved;
@override final  Object? actionError;

/// Create a copy of ResultState
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ResultStateCopyWith<_ResultState> get copyWith => __$ResultStateCopyWithImpl<_ResultState>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _ResultState&&(identical(other.consultation, consultation) || other.consultation == consultation)&&(identical(other.saving, saving) || other.saving == saving)&&(identical(other.saved, saved) || other.saved == saved)&&const DeepCollectionEquality().equals(other.actionError, actionError));
}


@override
int get hashCode => Object.hash(runtimeType,consultation,saving,saved,const DeepCollectionEquality().hash(actionError));

@override
String toString() {
  return 'ResultState(consultation: $consultation, saving: $saving, saved: $saved, actionError: $actionError)';
}


}

/// @nodoc
abstract mixin class _$ResultStateCopyWith<$Res> implements $ResultStateCopyWith<$Res> {
  factory _$ResultStateCopyWith(_ResultState value, $Res Function(_ResultState) _then) = __$ResultStateCopyWithImpl;
@override @useResult
$Res call({
 AsyncValue<Consultation?> consultation, bool saving, bool saved, Object? actionError
});




}
/// @nodoc
class __$ResultStateCopyWithImpl<$Res>
    implements _$ResultStateCopyWith<$Res> {
  __$ResultStateCopyWithImpl(this._self, this._then);

  final _ResultState _self;
  final $Res Function(_ResultState) _then;

/// Create a copy of ResultState
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? consultation = null,Object? saving = null,Object? saved = null,Object? actionError = freezed,}) {
  return _then(_ResultState(
consultation: null == consultation ? _self.consultation : consultation // ignore: cast_nullable_to_non_nullable
as AsyncValue<Consultation?>,saving: null == saving ? _self.saving : saving // ignore: cast_nullable_to_non_nullable
as bool,saved: null == saved ? _self.saved : saved // ignore: cast_nullable_to_non_nullable
as bool,actionError: freezed == actionError ? _self.actionError : actionError ,
  ));
}


}

// dart format on
