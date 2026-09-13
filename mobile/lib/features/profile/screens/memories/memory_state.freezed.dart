// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'memory_state.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$MemoryState {

 AsyncValue<List<MemoryItem>> get items; bool get saving; Object? get actionError;
/// Create a copy of MemoryState
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$MemoryStateCopyWith<MemoryState> get copyWith => _$MemoryStateCopyWithImpl<MemoryState>(this as MemoryState, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is MemoryState&&(identical(other.items, items) || other.items == items)&&(identical(other.saving, saving) || other.saving == saving)&&const DeepCollectionEquality().equals(other.actionError, actionError));
}


@override
int get hashCode => Object.hash(runtimeType,items,saving,const DeepCollectionEquality().hash(actionError));

@override
String toString() {
  return 'MemoryState(items: $items, saving: $saving, actionError: $actionError)';
}


}

/// @nodoc
abstract mixin class $MemoryStateCopyWith<$Res>  {
  factory $MemoryStateCopyWith(MemoryState value, $Res Function(MemoryState) _then) = _$MemoryStateCopyWithImpl;
@useResult
$Res call({
 AsyncValue<List<MemoryItem>> items, bool saving, Object? actionError
});




}
/// @nodoc
class _$MemoryStateCopyWithImpl<$Res>
    implements $MemoryStateCopyWith<$Res> {
  _$MemoryStateCopyWithImpl(this._self, this._then);

  final MemoryState _self;
  final $Res Function(MemoryState) _then;

/// Create a copy of MemoryState
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? items = null,Object? saving = null,Object? actionError = freezed,}) {
  return _then(_self.copyWith(
items: null == items ? _self.items : items // ignore: cast_nullable_to_non_nullable
as AsyncValue<List<MemoryItem>>,saving: null == saving ? _self.saving : saving // ignore: cast_nullable_to_non_nullable
as bool,actionError: freezed == actionError ? _self.actionError : actionError ,
  ));
}

}


/// Adds pattern-matching-related methods to [MemoryState].
extension MemoryStatePatterns on MemoryState {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _MemoryState value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _MemoryState() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _MemoryState value)  $default,){
final _that = this;
switch (_that) {
case _MemoryState():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _MemoryState value)?  $default,){
final _that = this;
switch (_that) {
case _MemoryState() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( AsyncValue<List<MemoryItem>> items,  bool saving,  Object? actionError)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _MemoryState() when $default != null:
return $default(_that.items,_that.saving,_that.actionError);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( AsyncValue<List<MemoryItem>> items,  bool saving,  Object? actionError)  $default,) {final _that = this;
switch (_that) {
case _MemoryState():
return $default(_that.items,_that.saving,_that.actionError);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( AsyncValue<List<MemoryItem>> items,  bool saving,  Object? actionError)?  $default,) {final _that = this;
switch (_that) {
case _MemoryState() when $default != null:
return $default(_that.items,_that.saving,_that.actionError);case _:
  return null;

}
}

}

/// @nodoc


class _MemoryState implements MemoryState {
  const _MemoryState({required this.items, this.saving = false, this.actionError});
  

@override final  AsyncValue<List<MemoryItem>> items;
@override@JsonKey() final  bool saving;
@override final  Object? actionError;

/// Create a copy of MemoryState
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$MemoryStateCopyWith<_MemoryState> get copyWith => __$MemoryStateCopyWithImpl<_MemoryState>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _MemoryState&&(identical(other.items, items) || other.items == items)&&(identical(other.saving, saving) || other.saving == saving)&&const DeepCollectionEquality().equals(other.actionError, actionError));
}


@override
int get hashCode => Object.hash(runtimeType,items,saving,const DeepCollectionEquality().hash(actionError));

@override
String toString() {
  return 'MemoryState(items: $items, saving: $saving, actionError: $actionError)';
}


}

/// @nodoc
abstract mixin class _$MemoryStateCopyWith<$Res> implements $MemoryStateCopyWith<$Res> {
  factory _$MemoryStateCopyWith(_MemoryState value, $Res Function(_MemoryState) _then) = __$MemoryStateCopyWithImpl;
@override @useResult
$Res call({
 AsyncValue<List<MemoryItem>> items, bool saving, Object? actionError
});




}
/// @nodoc
class __$MemoryStateCopyWithImpl<$Res>
    implements _$MemoryStateCopyWith<$Res> {
  __$MemoryStateCopyWithImpl(this._self, this._then);

  final _MemoryState _self;
  final $Res Function(_MemoryState) _then;

/// Create a copy of MemoryState
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? items = null,Object? saving = null,Object? actionError = freezed,}) {
  return _then(_MemoryState(
items: null == items ? _self.items : items // ignore: cast_nullable_to_non_nullable
as AsyncValue<List<MemoryItem>>,saving: null == saving ? _self.saving : saving // ignore: cast_nullable_to_non_nullable
as bool,actionError: freezed == actionError ? _self.actionError : actionError ,
  ));
}


}

// dart format on
