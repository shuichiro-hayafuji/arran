// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'result_intent.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$ResultIntent {





@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ResultIntent);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'ResultIntent()';
}


}

/// @nodoc
class $ResultIntentCopyWith<$Res>  {
$ResultIntentCopyWith(ResultIntent _, $Res Function(ResultIntent) __);
}


/// Adds pattern-matching-related methods to [ResultIntent].
extension ResultIntentPatterns on ResultIntent {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>({TResult Function( LoadResult value)?  load,TResult Function( SaveResult value)?  save,required TResult orElse(),}){
final _that = this;
switch (_that) {
case LoadResult() when load != null:
return load(_that);case SaveResult() when save != null:
return save(_that);case _:
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

@optionalTypeArgs TResult map<TResult extends Object?>({required TResult Function( LoadResult value)  load,required TResult Function( SaveResult value)  save,}){
final _that = this;
switch (_that) {
case LoadResult():
return load(_that);case SaveResult():
return save(_that);}
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>({TResult? Function( LoadResult value)?  load,TResult? Function( SaveResult value)?  save,}){
final _that = this;
switch (_that) {
case LoadResult() when load != null:
return load(_that);case SaveResult() when save != null:
return save(_that);case _:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>({TResult Function()?  load,TResult Function( String status,  int? actualAmount,  String reason,  int satisfaction,  int regret,  String note)?  save,required TResult orElse(),}) {final _that = this;
switch (_that) {
case LoadResult() when load != null:
return load();case SaveResult() when save != null:
return save(_that.status,_that.actualAmount,_that.reason,_that.satisfaction,_that.regret,_that.note);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>({required TResult Function()  load,required TResult Function( String status,  int? actualAmount,  String reason,  int satisfaction,  int regret,  String note)  save,}) {final _that = this;
switch (_that) {
case LoadResult():
return load();case SaveResult():
return save(_that.status,_that.actualAmount,_that.reason,_that.satisfaction,_that.regret,_that.note);}
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>({TResult? Function()?  load,TResult? Function( String status,  int? actualAmount,  String reason,  int satisfaction,  int regret,  String note)?  save,}) {final _that = this;
switch (_that) {
case LoadResult() when load != null:
return load();case SaveResult() when save != null:
return save(_that.status,_that.actualAmount,_that.reason,_that.satisfaction,_that.regret,_that.note);case _:
  return null;

}
}

}

/// @nodoc


class LoadResult extends ResultIntent {
  const LoadResult(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is LoadResult);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'ResultIntent.load()';
}


}




/// @nodoc


class SaveResult extends ResultIntent {
  const SaveResult({required this.status, required this.actualAmount, required this.reason, required this.satisfaction, required this.regret, required this.note}): super._();
  

 final  String status;
 final  int? actualAmount;
 final  String reason;
 final  int satisfaction;
 final  int regret;
 final  String note;

/// Create a copy of ResultIntent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SaveResultCopyWith<SaveResult> get copyWith => _$SaveResultCopyWithImpl<SaveResult>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SaveResult&&(identical(other.status, status) || other.status == status)&&(identical(other.actualAmount, actualAmount) || other.actualAmount == actualAmount)&&(identical(other.reason, reason) || other.reason == reason)&&(identical(other.satisfaction, satisfaction) || other.satisfaction == satisfaction)&&(identical(other.regret, regret) || other.regret == regret)&&(identical(other.note, note) || other.note == note));
}


@override
int get hashCode => Object.hash(runtimeType,status,actualAmount,reason,satisfaction,regret,note);

@override
String toString() {
  return 'ResultIntent.save(status: $status, actualAmount: $actualAmount, reason: $reason, satisfaction: $satisfaction, regret: $regret, note: $note)';
}


}

/// @nodoc
abstract mixin class $SaveResultCopyWith<$Res> implements $ResultIntentCopyWith<$Res> {
  factory $SaveResultCopyWith(SaveResult value, $Res Function(SaveResult) _then) = _$SaveResultCopyWithImpl;
@useResult
$Res call({
 String status, int? actualAmount, String reason, int satisfaction, int regret, String note
});




}
/// @nodoc
class _$SaveResultCopyWithImpl<$Res>
    implements $SaveResultCopyWith<$Res> {
  _$SaveResultCopyWithImpl(this._self, this._then);

  final SaveResult _self;
  final $Res Function(SaveResult) _then;

/// Create a copy of ResultIntent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? status = null,Object? actualAmount = freezed,Object? reason = null,Object? satisfaction = null,Object? regret = null,Object? note = null,}) {
  return _then(SaveResult(
status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,actualAmount: freezed == actualAmount ? _self.actualAmount : actualAmount // ignore: cast_nullable_to_non_nullable
as int?,reason: null == reason ? _self.reason : reason // ignore: cast_nullable_to_non_nullable
as String,satisfaction: null == satisfaction ? _self.satisfaction : satisfaction // ignore: cast_nullable_to_non_nullable
as int,regret: null == regret ? _self.regret : regret // ignore: cast_nullable_to_non_nullable
as int,note: null == note ? _self.note : note // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

// dart format on
