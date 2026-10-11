// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'consultation_intent.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$ConsultationIntent {





@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ConsultationIntent);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'ConsultationIntent()';
}


}

/// @nodoc
class $ConsultationIntentCopyWith<$Res>  {
$ConsultationIntentCopyWith(ConsultationIntent _, $Res Function(ConsultationIntent) __);
}


/// Adds pattern-matching-related methods to [ConsultationIntent].
extension ConsultationIntentPatterns on ConsultationIntent {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>({TResult Function( SubmitConsultation value)?  submit,TResult Function( ContinueConsultation value)?  continueWith,TResult Function( SelectConsultation value)?  select,TResult Function( ClearConsultation value)?  clear,TResult Function( RecoverConsultationRequest value)?  recoverRequest,TResult Function( CancelConsultationRequest value)?  cancelRequest,required TResult orElse(),}){
final _that = this;
switch (_that) {
case SubmitConsultation() when submit != null:
return submit(_that);case ContinueConsultation() when continueWith != null:
return continueWith(_that);case SelectConsultation() when select != null:
return select(_that);case ClearConsultation() when clear != null:
return clear(_that);case RecoverConsultationRequest() when recoverRequest != null:
return recoverRequest(_that);case CancelConsultationRequest() when cancelRequest != null:
return cancelRequest(_that);case _:
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

@optionalTypeArgs TResult map<TResult extends Object?>({required TResult Function( SubmitConsultation value)  submit,required TResult Function( ContinueConsultation value)  continueWith,required TResult Function( SelectConsultation value)  select,required TResult Function( ClearConsultation value)  clear,required TResult Function( RecoverConsultationRequest value)  recoverRequest,required TResult Function( CancelConsultationRequest value)  cancelRequest,}){
final _that = this;
switch (_that) {
case SubmitConsultation():
return submit(_that);case ContinueConsultation():
return continueWith(_that);case SelectConsultation():
return select(_that);case ClearConsultation():
return clear(_that);case RecoverConsultationRequest():
return recoverRequest(_that);case CancelConsultationRequest():
return cancelRequest(_that);}
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>({TResult? Function( SubmitConsultation value)?  submit,TResult? Function( ContinueConsultation value)?  continueWith,TResult? Function( SelectConsultation value)?  select,TResult? Function( ClearConsultation value)?  clear,TResult? Function( RecoverConsultationRequest value)?  recoverRequest,TResult? Function( CancelConsultationRequest value)?  cancelRequest,}){
final _that = this;
switch (_that) {
case SubmitConsultation() when submit != null:
return submit(_that);case ContinueConsultation() when continueWith != null:
return continueWith(_that);case SelectConsultation() when select != null:
return select(_that);case ClearConsultation() when clear != null:
return clear(_that);case RecoverConsultationRequest() when recoverRequest != null:
return recoverRequest(_that);case CancelConsultationRequest() when cancelRequest != null:
return cancelRequest(_that);case _:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>({TResult Function( String message,  int? plannedAmount)?  submit,TResult Function( int id,  String message,  int? plannedAmount)?  continueWith,TResult Function( Consultation consultation)?  select,TResult Function()?  clear,TResult Function()?  recoverRequest,TResult Function()?  cancelRequest,required TResult orElse(),}) {final _that = this;
switch (_that) {
case SubmitConsultation() when submit != null:
return submit(_that.message,_that.plannedAmount);case ContinueConsultation() when continueWith != null:
return continueWith(_that.id,_that.message,_that.plannedAmount);case SelectConsultation() when select != null:
return select(_that.consultation);case ClearConsultation() when clear != null:
return clear();case RecoverConsultationRequest() when recoverRequest != null:
return recoverRequest();case CancelConsultationRequest() when cancelRequest != null:
return cancelRequest();case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>({required TResult Function( String message,  int? plannedAmount)  submit,required TResult Function( int id,  String message,  int? plannedAmount)  continueWith,required TResult Function( Consultation consultation)  select,required TResult Function()  clear,required TResult Function()  recoverRequest,required TResult Function()  cancelRequest,}) {final _that = this;
switch (_that) {
case SubmitConsultation():
return submit(_that.message,_that.plannedAmount);case ContinueConsultation():
return continueWith(_that.id,_that.message,_that.plannedAmount);case SelectConsultation():
return select(_that.consultation);case ClearConsultation():
return clear();case RecoverConsultationRequest():
return recoverRequest();case CancelConsultationRequest():
return cancelRequest();}
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>({TResult? Function( String message,  int? plannedAmount)?  submit,TResult? Function( int id,  String message,  int? plannedAmount)?  continueWith,TResult? Function( Consultation consultation)?  select,TResult? Function()?  clear,TResult? Function()?  recoverRequest,TResult? Function()?  cancelRequest,}) {final _that = this;
switch (_that) {
case SubmitConsultation() when submit != null:
return submit(_that.message,_that.plannedAmount);case ContinueConsultation() when continueWith != null:
return continueWith(_that.id,_that.message,_that.plannedAmount);case SelectConsultation() when select != null:
return select(_that.consultation);case ClearConsultation() when clear != null:
return clear();case RecoverConsultationRequest() when recoverRequest != null:
return recoverRequest();case CancelConsultationRequest() when cancelRequest != null:
return cancelRequest();case _:
  return null;

}
}

}

/// @nodoc


class SubmitConsultation extends ConsultationIntent {
  const SubmitConsultation(this.message, this.plannedAmount): super._();
  

 final  String message;
 final  int? plannedAmount;

/// Create a copy of ConsultationIntent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SubmitConsultationCopyWith<SubmitConsultation> get copyWith => _$SubmitConsultationCopyWithImpl<SubmitConsultation>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SubmitConsultation&&(identical(other.message, message) || other.message == message)&&(identical(other.plannedAmount, plannedAmount) || other.plannedAmount == plannedAmount));
}


@override
int get hashCode => Object.hash(runtimeType,message,plannedAmount);

@override
String toString() {
  return 'ConsultationIntent.submit(message: $message, plannedAmount: $plannedAmount)';
}


}

/// @nodoc
abstract mixin class $SubmitConsultationCopyWith<$Res> implements $ConsultationIntentCopyWith<$Res> {
  factory $SubmitConsultationCopyWith(SubmitConsultation value, $Res Function(SubmitConsultation) _then) = _$SubmitConsultationCopyWithImpl;
@useResult
$Res call({
 String message, int? plannedAmount
});




}
/// @nodoc
class _$SubmitConsultationCopyWithImpl<$Res>
    implements $SubmitConsultationCopyWith<$Res> {
  _$SubmitConsultationCopyWithImpl(this._self, this._then);

  final SubmitConsultation _self;
  final $Res Function(SubmitConsultation) _then;

/// Create a copy of ConsultationIntent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? message = null,Object? plannedAmount = freezed,}) {
  return _then(SubmitConsultation(
null == message ? _self.message : message // ignore: cast_nullable_to_non_nullable
as String,freezed == plannedAmount ? _self.plannedAmount : plannedAmount // ignore: cast_nullable_to_non_nullable
as int?,
  ));
}


}

/// @nodoc


class ContinueConsultation extends ConsultationIntent {
  const ContinueConsultation(this.id, this.message, this.plannedAmount): super._();
  

 final  int id;
 final  String message;
 final  int? plannedAmount;

/// Create a copy of ConsultationIntent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ContinueConsultationCopyWith<ContinueConsultation> get copyWith => _$ContinueConsultationCopyWithImpl<ContinueConsultation>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ContinueConsultation&&(identical(other.id, id) || other.id == id)&&(identical(other.message, message) || other.message == message)&&(identical(other.plannedAmount, plannedAmount) || other.plannedAmount == plannedAmount));
}


@override
int get hashCode => Object.hash(runtimeType,id,message,plannedAmount);

@override
String toString() {
  return 'ConsultationIntent.continueWith(id: $id, message: $message, plannedAmount: $plannedAmount)';
}


}

/// @nodoc
abstract mixin class $ContinueConsultationCopyWith<$Res> implements $ConsultationIntentCopyWith<$Res> {
  factory $ContinueConsultationCopyWith(ContinueConsultation value, $Res Function(ContinueConsultation) _then) = _$ContinueConsultationCopyWithImpl;
@useResult
$Res call({
 int id, String message, int? plannedAmount
});




}
/// @nodoc
class _$ContinueConsultationCopyWithImpl<$Res>
    implements $ContinueConsultationCopyWith<$Res> {
  _$ContinueConsultationCopyWithImpl(this._self, this._then);

  final ContinueConsultation _self;
  final $Res Function(ContinueConsultation) _then;

/// Create a copy of ConsultationIntent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? id = null,Object? message = null,Object? plannedAmount = freezed,}) {
  return _then(ContinueConsultation(
null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,null == message ? _self.message : message // ignore: cast_nullable_to_non_nullable
as String,freezed == plannedAmount ? _self.plannedAmount : plannedAmount // ignore: cast_nullable_to_non_nullable
as int?,
  ));
}


}

/// @nodoc


class SelectConsultation extends ConsultationIntent {
  const SelectConsultation(this.consultation): super._();
  

 final  Consultation consultation;

/// Create a copy of ConsultationIntent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SelectConsultationCopyWith<SelectConsultation> get copyWith => _$SelectConsultationCopyWithImpl<SelectConsultation>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SelectConsultation&&(identical(other.consultation, consultation) || other.consultation == consultation));
}


@override
int get hashCode => Object.hash(runtimeType,consultation);

@override
String toString() {
  return 'ConsultationIntent.select(consultation: $consultation)';
}


}

/// @nodoc
abstract mixin class $SelectConsultationCopyWith<$Res> implements $ConsultationIntentCopyWith<$Res> {
  factory $SelectConsultationCopyWith(SelectConsultation value, $Res Function(SelectConsultation) _then) = _$SelectConsultationCopyWithImpl;
@useResult
$Res call({
 Consultation consultation
});




}
/// @nodoc
class _$SelectConsultationCopyWithImpl<$Res>
    implements $SelectConsultationCopyWith<$Res> {
  _$SelectConsultationCopyWithImpl(this._self, this._then);

  final SelectConsultation _self;
  final $Res Function(SelectConsultation) _then;

/// Create a copy of ConsultationIntent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? consultation = null,}) {
  return _then(SelectConsultation(
null == consultation ? _self.consultation : consultation // ignore: cast_nullable_to_non_nullable
as Consultation,
  ));
}


}

/// @nodoc


class ClearConsultation extends ConsultationIntent {
  const ClearConsultation(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ClearConsultation);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'ConsultationIntent.clear()';
}


}




/// @nodoc


class RecoverConsultationRequest extends ConsultationIntent {
  const RecoverConsultationRequest(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is RecoverConsultationRequest);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'ConsultationIntent.recoverRequest()';
}


}




/// @nodoc


class CancelConsultationRequest extends ConsultationIntent {
  const CancelConsultationRequest(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is CancelConsultationRequest);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'ConsultationIntent.cancelRequest()';
}


}




// dart format on
