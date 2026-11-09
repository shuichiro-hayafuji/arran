// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'import_intent.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$ImportIntent {





@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ImportIntent);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'ImportIntent()';
}


}

/// @nodoc
class $ImportIntentCopyWith<$Res>  {
$ImportIntentCopyWith(ImportIntent _, $Res Function(ImportIntent) __);
}


/// Adds pattern-matching-related methods to [ImportIntent].
extension ImportIntentPatterns on ImportIntent {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>({TResult Function( SelectImportFile value)?  selectFile,TResult Function( ApplyImportMapping value)?  applyMapping,TResult Function( SetImportSource value)?  setSource,TResult Function( ResolveImportIdentities value)?  resolveIdentities,TResult Function( ResolveImportPurchases value)?  resolvePurchases,TResult Function( CommitImport value)?  commit,TResult Function( ResetImport value)?  reset,required TResult orElse(),}){
final _that = this;
switch (_that) {
case SelectImportFile() when selectFile != null:
return selectFile(_that);case ApplyImportMapping() when applyMapping != null:
return applyMapping(_that);case SetImportSource() when setSource != null:
return setSource(_that);case ResolveImportIdentities() when resolveIdentities != null:
return resolveIdentities(_that);case ResolveImportPurchases() when resolvePurchases != null:
return resolvePurchases(_that);case CommitImport() when commit != null:
return commit(_that);case ResetImport() when reset != null:
return reset(_that);case _:
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

@optionalTypeArgs TResult map<TResult extends Object?>({required TResult Function( SelectImportFile value)  selectFile,required TResult Function( ApplyImportMapping value)  applyMapping,required TResult Function( SetImportSource value)  setSource,required TResult Function( ResolveImportIdentities value)  resolveIdentities,required TResult Function( ResolveImportPurchases value)  resolvePurchases,required TResult Function( CommitImport value)  commit,required TResult Function( ResetImport value)  reset,}){
final _that = this;
switch (_that) {
case SelectImportFile():
return selectFile(_that);case ApplyImportMapping():
return applyMapping(_that);case SetImportSource():
return setSource(_that);case ResolveImportIdentities():
return resolveIdentities(_that);case ResolveImportPurchases():
return resolvePurchases(_that);case CommitImport():
return commit(_that);case ResetImport():
return reset(_that);}
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>({TResult? Function( SelectImportFile value)?  selectFile,TResult? Function( ApplyImportMapping value)?  applyMapping,TResult? Function( SetImportSource value)?  setSource,TResult? Function( ResolveImportIdentities value)?  resolveIdentities,TResult? Function( ResolveImportPurchases value)?  resolvePurchases,TResult? Function( CommitImport value)?  commit,TResult? Function( ResetImport value)?  reset,}){
final _that = this;
switch (_that) {
case SelectImportFile() when selectFile != null:
return selectFile(_that);case ApplyImportMapping() when applyMapping != null:
return applyMapping(_that);case SetImportSource() when setSource != null:
return setSource(_that);case ResolveImportIdentities() when resolveIdentities != null:
return resolveIdentities(_that);case ResolveImportPurchases() when resolvePurchases != null:
return resolvePurchases(_that);case CommitImport() when commit != null:
return commit(_that);case ResetImport() when reset != null:
return reset(_that);case _:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>({TResult Function( String filePath,  String fileName)?  selectFile,TResult Function( CsvMapping mapping)?  applyMapping,TResult Function( String source,  String accountName)?  setSource,TResult Function( List<ImportIdentityDecision> decisions)?  resolveIdentities,TResult Function( List<PurchaseCsvDecision> decisions)?  resolvePurchases,TResult Function()?  commit,TResult Function()?  reset,required TResult orElse(),}) {final _that = this;
switch (_that) {
case SelectImportFile() when selectFile != null:
return selectFile(_that.filePath,_that.fileName);case ApplyImportMapping() when applyMapping != null:
return applyMapping(_that.mapping);case SetImportSource() when setSource != null:
return setSource(_that.source,_that.accountName);case ResolveImportIdentities() when resolveIdentities != null:
return resolveIdentities(_that.decisions);case ResolveImportPurchases() when resolvePurchases != null:
return resolvePurchases(_that.decisions);case CommitImport() when commit != null:
return commit();case ResetImport() when reset != null:
return reset();case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>({required TResult Function( String filePath,  String fileName)  selectFile,required TResult Function( CsvMapping mapping)  applyMapping,required TResult Function( String source,  String accountName)  setSource,required TResult Function( List<ImportIdentityDecision> decisions)  resolveIdentities,required TResult Function( List<PurchaseCsvDecision> decisions)  resolvePurchases,required TResult Function()  commit,required TResult Function()  reset,}) {final _that = this;
switch (_that) {
case SelectImportFile():
return selectFile(_that.filePath,_that.fileName);case ApplyImportMapping():
return applyMapping(_that.mapping);case SetImportSource():
return setSource(_that.source,_that.accountName);case ResolveImportIdentities():
return resolveIdentities(_that.decisions);case ResolveImportPurchases():
return resolvePurchases(_that.decisions);case CommitImport():
return commit();case ResetImport():
return reset();}
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>({TResult? Function( String filePath,  String fileName)?  selectFile,TResult? Function( CsvMapping mapping)?  applyMapping,TResult? Function( String source,  String accountName)?  setSource,TResult? Function( List<ImportIdentityDecision> decisions)?  resolveIdentities,TResult? Function( List<PurchaseCsvDecision> decisions)?  resolvePurchases,TResult? Function()?  commit,TResult? Function()?  reset,}) {final _that = this;
switch (_that) {
case SelectImportFile() when selectFile != null:
return selectFile(_that.filePath,_that.fileName);case ApplyImportMapping() when applyMapping != null:
return applyMapping(_that.mapping);case SetImportSource() when setSource != null:
return setSource(_that.source,_that.accountName);case ResolveImportIdentities() when resolveIdentities != null:
return resolveIdentities(_that.decisions);case ResolveImportPurchases() when resolvePurchases != null:
return resolvePurchases(_that.decisions);case CommitImport() when commit != null:
return commit();case ResetImport() when reset != null:
return reset();case _:
  return null;

}
}

}

/// @nodoc


class SelectImportFile extends ImportIntent {
  const SelectImportFile({required this.filePath, required this.fileName}): super._();
  

 final  String filePath;
 final  String fileName;

/// Create a copy of ImportIntent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SelectImportFileCopyWith<SelectImportFile> get copyWith => _$SelectImportFileCopyWithImpl<SelectImportFile>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SelectImportFile&&(identical(other.filePath, filePath) || other.filePath == filePath)&&(identical(other.fileName, fileName) || other.fileName == fileName));
}


@override
int get hashCode => Object.hash(runtimeType,filePath,fileName);

@override
String toString() {
  return 'ImportIntent.selectFile(filePath: $filePath, fileName: $fileName)';
}


}

/// @nodoc
abstract mixin class $SelectImportFileCopyWith<$Res> implements $ImportIntentCopyWith<$Res> {
  factory $SelectImportFileCopyWith(SelectImportFile value, $Res Function(SelectImportFile) _then) = _$SelectImportFileCopyWithImpl;
@useResult
$Res call({
 String filePath, String fileName
});




}
/// @nodoc
class _$SelectImportFileCopyWithImpl<$Res>
    implements $SelectImportFileCopyWith<$Res> {
  _$SelectImportFileCopyWithImpl(this._self, this._then);

  final SelectImportFile _self;
  final $Res Function(SelectImportFile) _then;

/// Create a copy of ImportIntent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? filePath = null,Object? fileName = null,}) {
  return _then(SelectImportFile(
filePath: null == filePath ? _self.filePath : filePath // ignore: cast_nullable_to_non_nullable
as String,fileName: null == fileName ? _self.fileName : fileName // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

/// @nodoc


class ApplyImportMapping extends ImportIntent {
  const ApplyImportMapping(this.mapping): super._();
  

 final  CsvMapping mapping;

/// Create a copy of ImportIntent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ApplyImportMappingCopyWith<ApplyImportMapping> get copyWith => _$ApplyImportMappingCopyWithImpl<ApplyImportMapping>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ApplyImportMapping&&(identical(other.mapping, mapping) || other.mapping == mapping));
}


@override
int get hashCode => Object.hash(runtimeType,mapping);

@override
String toString() {
  return 'ImportIntent.applyMapping(mapping: $mapping)';
}


}

/// @nodoc
abstract mixin class $ApplyImportMappingCopyWith<$Res> implements $ImportIntentCopyWith<$Res> {
  factory $ApplyImportMappingCopyWith(ApplyImportMapping value, $Res Function(ApplyImportMapping) _then) = _$ApplyImportMappingCopyWithImpl;
@useResult
$Res call({
 CsvMapping mapping
});




}
/// @nodoc
class _$ApplyImportMappingCopyWithImpl<$Res>
    implements $ApplyImportMappingCopyWith<$Res> {
  _$ApplyImportMappingCopyWithImpl(this._self, this._then);

  final ApplyImportMapping _self;
  final $Res Function(ApplyImportMapping) _then;

/// Create a copy of ImportIntent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? mapping = null,}) {
  return _then(ApplyImportMapping(
null == mapping ? _self.mapping : mapping // ignore: cast_nullable_to_non_nullable
as CsvMapping,
  ));
}


}

/// @nodoc


class SetImportSource extends ImportIntent {
  const SetImportSource(this.source, this.accountName): super._();
  

 final  String source;
 final  String accountName;

/// Create a copy of ImportIntent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SetImportSourceCopyWith<SetImportSource> get copyWith => _$SetImportSourceCopyWithImpl<SetImportSource>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SetImportSource&&(identical(other.source, source) || other.source == source)&&(identical(other.accountName, accountName) || other.accountName == accountName));
}


@override
int get hashCode => Object.hash(runtimeType,source,accountName);

@override
String toString() {
  return 'ImportIntent.setSource(source: $source, accountName: $accountName)';
}


}

/// @nodoc
abstract mixin class $SetImportSourceCopyWith<$Res> implements $ImportIntentCopyWith<$Res> {
  factory $SetImportSourceCopyWith(SetImportSource value, $Res Function(SetImportSource) _then) = _$SetImportSourceCopyWithImpl;
@useResult
$Res call({
 String source, String accountName
});




}
/// @nodoc
class _$SetImportSourceCopyWithImpl<$Res>
    implements $SetImportSourceCopyWith<$Res> {
  _$SetImportSourceCopyWithImpl(this._self, this._then);

  final SetImportSource _self;
  final $Res Function(SetImportSource) _then;

/// Create a copy of ImportIntent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? source = null,Object? accountName = null,}) {
  return _then(SetImportSource(
null == source ? _self.source : source // ignore: cast_nullable_to_non_nullable
as String,null == accountName ? _self.accountName : accountName // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

/// @nodoc


class ResolveImportIdentities extends ImportIntent {
  const ResolveImportIdentities(final  List<ImportIdentityDecision> decisions): _decisions = decisions,super._();
  

 final  List<ImportIdentityDecision> _decisions;
 List<ImportIdentityDecision> get decisions {
  if (_decisions is EqualUnmodifiableListView) return _decisions;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_decisions);
}


/// Create a copy of ImportIntent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ResolveImportIdentitiesCopyWith<ResolveImportIdentities> get copyWith => _$ResolveImportIdentitiesCopyWithImpl<ResolveImportIdentities>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ResolveImportIdentities&&const DeepCollectionEquality().equals(other._decisions, _decisions));
}


@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(_decisions));

@override
String toString() {
  return 'ImportIntent.resolveIdentities(decisions: $decisions)';
}


}

/// @nodoc
abstract mixin class $ResolveImportIdentitiesCopyWith<$Res> implements $ImportIntentCopyWith<$Res> {
  factory $ResolveImportIdentitiesCopyWith(ResolveImportIdentities value, $Res Function(ResolveImportIdentities) _then) = _$ResolveImportIdentitiesCopyWithImpl;
@useResult
$Res call({
 List<ImportIdentityDecision> decisions
});




}
/// @nodoc
class _$ResolveImportIdentitiesCopyWithImpl<$Res>
    implements $ResolveImportIdentitiesCopyWith<$Res> {
  _$ResolveImportIdentitiesCopyWithImpl(this._self, this._then);

  final ResolveImportIdentities _self;
  final $Res Function(ResolveImportIdentities) _then;

/// Create a copy of ImportIntent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? decisions = null,}) {
  return _then(ResolveImportIdentities(
null == decisions ? _self._decisions : decisions // ignore: cast_nullable_to_non_nullable
as List<ImportIdentityDecision>,
  ));
}


}

/// @nodoc


class ResolveImportPurchases extends ImportIntent {
  const ResolveImportPurchases(final  List<PurchaseCsvDecision> decisions): _decisions = decisions,super._();
  

 final  List<PurchaseCsvDecision> _decisions;
 List<PurchaseCsvDecision> get decisions {
  if (_decisions is EqualUnmodifiableListView) return _decisions;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_decisions);
}


/// Create a copy of ImportIntent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ResolveImportPurchasesCopyWith<ResolveImportPurchases> get copyWith => _$ResolveImportPurchasesCopyWithImpl<ResolveImportPurchases>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ResolveImportPurchases&&const DeepCollectionEquality().equals(other._decisions, _decisions));
}


@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(_decisions));

@override
String toString() {
  return 'ImportIntent.resolvePurchases(decisions: $decisions)';
}


}

/// @nodoc
abstract mixin class $ResolveImportPurchasesCopyWith<$Res> implements $ImportIntentCopyWith<$Res> {
  factory $ResolveImportPurchasesCopyWith(ResolveImportPurchases value, $Res Function(ResolveImportPurchases) _then) = _$ResolveImportPurchasesCopyWithImpl;
@useResult
$Res call({
 List<PurchaseCsvDecision> decisions
});




}
/// @nodoc
class _$ResolveImportPurchasesCopyWithImpl<$Res>
    implements $ResolveImportPurchasesCopyWith<$Res> {
  _$ResolveImportPurchasesCopyWithImpl(this._self, this._then);

  final ResolveImportPurchases _self;
  final $Res Function(ResolveImportPurchases) _then;

/// Create a copy of ImportIntent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? decisions = null,}) {
  return _then(ResolveImportPurchases(
null == decisions ? _self._decisions : decisions // ignore: cast_nullable_to_non_nullable
as List<PurchaseCsvDecision>,
  ));
}


}

/// @nodoc


class CommitImport extends ImportIntent {
  const CommitImport(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is CommitImport);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'ImportIntent.commit()';
}


}




/// @nodoc


class ResetImport extends ImportIntent {
  const ResetImport(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ResetImport);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'ImportIntent.reset()';
}


}




// dart format on
