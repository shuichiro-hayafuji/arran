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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>({TResult Function( SelectImportFile value)?  selectFile,TResult Function( ApplyImportMapping value)?  applyMapping,TResult Function( CommitImport value)?  commit,TResult Function( ResetImport value)?  reset,required TResult orElse(),}){
final _that = this;
switch (_that) {
case SelectImportFile() when selectFile != null:
return selectFile(_that);case ApplyImportMapping() when applyMapping != null:
return applyMapping(_that);case CommitImport() when commit != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>({required TResult Function( SelectImportFile value)  selectFile,required TResult Function( ApplyImportMapping value)  applyMapping,required TResult Function( CommitImport value)  commit,required TResult Function( ResetImport value)  reset,}){
final _that = this;
switch (_that) {
case SelectImportFile():
return selectFile(_that);case ApplyImportMapping():
return applyMapping(_that);case CommitImport():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>({TResult? Function( SelectImportFile value)?  selectFile,TResult? Function( ApplyImportMapping value)?  applyMapping,TResult? Function( CommitImport value)?  commit,TResult? Function( ResetImport value)?  reset,}){
final _that = this;
switch (_that) {
case SelectImportFile() when selectFile != null:
return selectFile(_that);case ApplyImportMapping() when applyMapping != null:
return applyMapping(_that);case CommitImport() when commit != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>({TResult Function( String filePath,  String fileName)?  selectFile,TResult Function( CsvMapping mapping)?  applyMapping,TResult Function()?  commit,TResult Function()?  reset,required TResult orElse(),}) {final _that = this;
switch (_that) {
case SelectImportFile() when selectFile != null:
return selectFile(_that.filePath,_that.fileName);case ApplyImportMapping() when applyMapping != null:
return applyMapping(_that.mapping);case CommitImport() when commit != null:
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

@optionalTypeArgs TResult when<TResult extends Object?>({required TResult Function( String filePath,  String fileName)  selectFile,required TResult Function( CsvMapping mapping)  applyMapping,required TResult Function()  commit,required TResult Function()  reset,}) {final _that = this;
switch (_that) {
case SelectImportFile():
return selectFile(_that.filePath,_that.fileName);case ApplyImportMapping():
return applyMapping(_that.mapping);case CommitImport():
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>({TResult? Function( String filePath,  String fileName)?  selectFile,TResult? Function( CsvMapping mapping)?  applyMapping,TResult? Function()?  commit,TResult? Function()?  reset,}) {final _that = this;
switch (_that) {
case SelectImportFile() when selectFile != null:
return selectFile(_that.filePath,_that.fileName);case ApplyImportMapping() when applyMapping != null:
return applyMapping(_that.mapping);case CommitImport() when commit != null:
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
