// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'import_view_model.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$ImportState {

 bool get confirmReactivation; String get source; String get accountName; List<ImportIdentityDecision> get identityDecisions; List<PurchaseCsvDecision> get reconciliationDecisions; String? get filePath; String? get fileName; ImportPreview? get preview; ImportCommitResult? get commitResult; bool get loading; String? get error;
/// Create a copy of ImportState
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ImportStateCopyWith<ImportState> get copyWith => _$ImportStateCopyWithImpl<ImportState>(this as ImportState, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ImportState&&(identical(other.confirmReactivation, confirmReactivation) || other.confirmReactivation == confirmReactivation)&&(identical(other.source, source) || other.source == source)&&(identical(other.accountName, accountName) || other.accountName == accountName)&&const DeepCollectionEquality().equals(other.identityDecisions, identityDecisions)&&const DeepCollectionEquality().equals(other.reconciliationDecisions, reconciliationDecisions)&&(identical(other.filePath, filePath) || other.filePath == filePath)&&(identical(other.fileName, fileName) || other.fileName == fileName)&&(identical(other.preview, preview) || other.preview == preview)&&(identical(other.commitResult, commitResult) || other.commitResult == commitResult)&&(identical(other.loading, loading) || other.loading == loading)&&(identical(other.error, error) || other.error == error));
}


@override
int get hashCode => Object.hash(runtimeType,confirmReactivation,source,accountName,const DeepCollectionEquality().hash(identityDecisions),const DeepCollectionEquality().hash(reconciliationDecisions),filePath,fileName,preview,commitResult,loading,error);

@override
String toString() {
  return 'ImportState(confirmReactivation: $confirmReactivation, source: $source, accountName: $accountName, identityDecisions: $identityDecisions, reconciliationDecisions: $reconciliationDecisions, filePath: $filePath, fileName: $fileName, preview: $preview, commitResult: $commitResult, loading: $loading, error: $error)';
}


}

/// @nodoc
abstract mixin class $ImportStateCopyWith<$Res>  {
  factory $ImportStateCopyWith(ImportState value, $Res Function(ImportState) _then) = _$ImportStateCopyWithImpl;
@useResult
$Res call({
 bool confirmReactivation, String source, String accountName, List<ImportIdentityDecision> identityDecisions, List<PurchaseCsvDecision> reconciliationDecisions, String? filePath, String? fileName, ImportPreview? preview, ImportCommitResult? commitResult, bool loading, String? error
});




}
/// @nodoc
class _$ImportStateCopyWithImpl<$Res>
    implements $ImportStateCopyWith<$Res> {
  _$ImportStateCopyWithImpl(this._self, this._then);

  final ImportState _self;
  final $Res Function(ImportState) _then;

/// Create a copy of ImportState
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? confirmReactivation = null,Object? source = null,Object? accountName = null,Object? identityDecisions = null,Object? reconciliationDecisions = null,Object? filePath = freezed,Object? fileName = freezed,Object? preview = freezed,Object? commitResult = freezed,Object? loading = null,Object? error = freezed,}) {
  return _then(_self.copyWith(
confirmReactivation: null == confirmReactivation ? _self.confirmReactivation : confirmReactivation // ignore: cast_nullable_to_non_nullable
as bool,source: null == source ? _self.source : source // ignore: cast_nullable_to_non_nullable
as String,accountName: null == accountName ? _self.accountName : accountName // ignore: cast_nullable_to_non_nullable
as String,identityDecisions: null == identityDecisions ? _self.identityDecisions : identityDecisions // ignore: cast_nullable_to_non_nullable
as List<ImportIdentityDecision>,reconciliationDecisions: null == reconciliationDecisions ? _self.reconciliationDecisions : reconciliationDecisions // ignore: cast_nullable_to_non_nullable
as List<PurchaseCsvDecision>,filePath: freezed == filePath ? _self.filePath : filePath // ignore: cast_nullable_to_non_nullable
as String?,fileName: freezed == fileName ? _self.fileName : fileName // ignore: cast_nullable_to_non_nullable
as String?,preview: freezed == preview ? _self.preview : preview // ignore: cast_nullable_to_non_nullable
as ImportPreview?,commitResult: freezed == commitResult ? _self.commitResult : commitResult // ignore: cast_nullable_to_non_nullable
as ImportCommitResult?,loading: null == loading ? _self.loading : loading // ignore: cast_nullable_to_non_nullable
as bool,error: freezed == error ? _self.error : error // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

}


/// Adds pattern-matching-related methods to [ImportState].
extension ImportStatePatterns on ImportState {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _ImportState value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _ImportState() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _ImportState value)  $default,){
final _that = this;
switch (_that) {
case _ImportState():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _ImportState value)?  $default,){
final _that = this;
switch (_that) {
case _ImportState() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( bool confirmReactivation,  String source,  String accountName,  List<ImportIdentityDecision> identityDecisions,  List<PurchaseCsvDecision> reconciliationDecisions,  String? filePath,  String? fileName,  ImportPreview? preview,  ImportCommitResult? commitResult,  bool loading,  String? error)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _ImportState() when $default != null:
return $default(_that.confirmReactivation,_that.source,_that.accountName,_that.identityDecisions,_that.reconciliationDecisions,_that.filePath,_that.fileName,_that.preview,_that.commitResult,_that.loading,_that.error);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( bool confirmReactivation,  String source,  String accountName,  List<ImportIdentityDecision> identityDecisions,  List<PurchaseCsvDecision> reconciliationDecisions,  String? filePath,  String? fileName,  ImportPreview? preview,  ImportCommitResult? commitResult,  bool loading,  String? error)  $default,) {final _that = this;
switch (_that) {
case _ImportState():
return $default(_that.confirmReactivation,_that.source,_that.accountName,_that.identityDecisions,_that.reconciliationDecisions,_that.filePath,_that.fileName,_that.preview,_that.commitResult,_that.loading,_that.error);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( bool confirmReactivation,  String source,  String accountName,  List<ImportIdentityDecision> identityDecisions,  List<PurchaseCsvDecision> reconciliationDecisions,  String? filePath,  String? fileName,  ImportPreview? preview,  ImportCommitResult? commitResult,  bool loading,  String? error)?  $default,) {final _that = this;
switch (_that) {
case _ImportState() when $default != null:
return $default(_that.confirmReactivation,_that.source,_that.accountName,_that.identityDecisions,_that.reconciliationDecisions,_that.filePath,_that.fileName,_that.preview,_that.commitResult,_that.loading,_that.error);case _:
  return null;

}
}

}

/// @nodoc


class _ImportState implements ImportState {
  const _ImportState({this.confirmReactivation = false, this.source = "manual_csv", this.accountName = "個人", final  List<ImportIdentityDecision> identityDecisions = const <ImportIdentityDecision>[], final  List<PurchaseCsvDecision> reconciliationDecisions = const <PurchaseCsvDecision>[], this.filePath, this.fileName, this.preview, this.commitResult, this.loading = false, this.error}): _identityDecisions = identityDecisions,_reconciliationDecisions = reconciliationDecisions;
  

@override@JsonKey() final  bool confirmReactivation;
@override@JsonKey() final  String source;
@override@JsonKey() final  String accountName;
 final  List<ImportIdentityDecision> _identityDecisions;
@override@JsonKey() List<ImportIdentityDecision> get identityDecisions {
  if (_identityDecisions is EqualUnmodifiableListView) return _identityDecisions;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_identityDecisions);
}

 final  List<PurchaseCsvDecision> _reconciliationDecisions;
@override@JsonKey() List<PurchaseCsvDecision> get reconciliationDecisions {
  if (_reconciliationDecisions is EqualUnmodifiableListView) return _reconciliationDecisions;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_reconciliationDecisions);
}

@override final  String? filePath;
@override final  String? fileName;
@override final  ImportPreview? preview;
@override final  ImportCommitResult? commitResult;
@override@JsonKey() final  bool loading;
@override final  String? error;

/// Create a copy of ImportState
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$ImportStateCopyWith<_ImportState> get copyWith => __$ImportStateCopyWithImpl<_ImportState>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _ImportState&&(identical(other.confirmReactivation, confirmReactivation) || other.confirmReactivation == confirmReactivation)&&(identical(other.source, source) || other.source == source)&&(identical(other.accountName, accountName) || other.accountName == accountName)&&const DeepCollectionEquality().equals(other._identityDecisions, _identityDecisions)&&const DeepCollectionEquality().equals(other._reconciliationDecisions, _reconciliationDecisions)&&(identical(other.filePath, filePath) || other.filePath == filePath)&&(identical(other.fileName, fileName) || other.fileName == fileName)&&(identical(other.preview, preview) || other.preview == preview)&&(identical(other.commitResult, commitResult) || other.commitResult == commitResult)&&(identical(other.loading, loading) || other.loading == loading)&&(identical(other.error, error) || other.error == error));
}


@override
int get hashCode => Object.hash(runtimeType,confirmReactivation,source,accountName,const DeepCollectionEquality().hash(_identityDecisions),const DeepCollectionEquality().hash(_reconciliationDecisions),filePath,fileName,preview,commitResult,loading,error);

@override
String toString() {
  return 'ImportState(confirmReactivation: $confirmReactivation, source: $source, accountName: $accountName, identityDecisions: $identityDecisions, reconciliationDecisions: $reconciliationDecisions, filePath: $filePath, fileName: $fileName, preview: $preview, commitResult: $commitResult, loading: $loading, error: $error)';
}


}

/// @nodoc
abstract mixin class _$ImportStateCopyWith<$Res> implements $ImportStateCopyWith<$Res> {
  factory _$ImportStateCopyWith(_ImportState value, $Res Function(_ImportState) _then) = __$ImportStateCopyWithImpl;
@override @useResult
$Res call({
 bool confirmReactivation, String source, String accountName, List<ImportIdentityDecision> identityDecisions, List<PurchaseCsvDecision> reconciliationDecisions, String? filePath, String? fileName, ImportPreview? preview, ImportCommitResult? commitResult, bool loading, String? error
});




}
/// @nodoc
class __$ImportStateCopyWithImpl<$Res>
    implements _$ImportStateCopyWith<$Res> {
  __$ImportStateCopyWithImpl(this._self, this._then);

  final _ImportState _self;
  final $Res Function(_ImportState) _then;

/// Create a copy of ImportState
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? confirmReactivation = null,Object? source = null,Object? accountName = null,Object? identityDecisions = null,Object? reconciliationDecisions = null,Object? filePath = freezed,Object? fileName = freezed,Object? preview = freezed,Object? commitResult = freezed,Object? loading = null,Object? error = freezed,}) {
  return _then(_ImportState(
confirmReactivation: null == confirmReactivation ? _self.confirmReactivation : confirmReactivation // ignore: cast_nullable_to_non_nullable
as bool,source: null == source ? _self.source : source // ignore: cast_nullable_to_non_nullable
as String,accountName: null == accountName ? _self.accountName : accountName // ignore: cast_nullable_to_non_nullable
as String,identityDecisions: null == identityDecisions ? _self._identityDecisions : identityDecisions // ignore: cast_nullable_to_non_nullable
as List<ImportIdentityDecision>,reconciliationDecisions: null == reconciliationDecisions ? _self._reconciliationDecisions : reconciliationDecisions // ignore: cast_nullable_to_non_nullable
as List<PurchaseCsvDecision>,filePath: freezed == filePath ? _self.filePath : filePath // ignore: cast_nullable_to_non_nullable
as String?,fileName: freezed == fileName ? _self.fileName : fileName // ignore: cast_nullable_to_non_nullable
as String?,preview: freezed == preview ? _self.preview : preview // ignore: cast_nullable_to_non_nullable
as ImportPreview?,commitResult: freezed == commitResult ? _self.commitResult : commitResult // ignore: cast_nullable_to_non_nullable
as ImportCommitResult?,loading: null == loading ? _self.loading : loading // ignore: cast_nullable_to_non_nullable
as bool,error: freezed == error ? _self.error : error // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}

// dart format on
