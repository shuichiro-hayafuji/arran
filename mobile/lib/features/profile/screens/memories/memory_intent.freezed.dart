// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'memory_intent.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$MemoryIntent {





@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is MemoryIntent);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'MemoryIntent()';
}


}

/// @nodoc
class $MemoryIntentCopyWith<$Res>  {
$MemoryIntentCopyWith(MemoryIntent _, $Res Function(MemoryIntent) __);
}


/// Adds pattern-matching-related methods to [MemoryIntent].
extension MemoryIntentPatterns on MemoryIntent {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>({TResult Function( LoadMemories value)?  load,TResult Function( RefreshMemories value)?  refresh,TResult Function( SaveMemory value)?  save,TResult Function( DeleteMemory value)?  delete,required TResult orElse(),}){
final _that = this;
switch (_that) {
case LoadMemories() when load != null:
return load(_that);case RefreshMemories() when refresh != null:
return refresh(_that);case SaveMemory() when save != null:
return save(_that);case DeleteMemory() when delete != null:
return delete(_that);case _:
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

@optionalTypeArgs TResult map<TResult extends Object?>({required TResult Function( LoadMemories value)  load,required TResult Function( RefreshMemories value)  refresh,required TResult Function( SaveMemory value)  save,required TResult Function( DeleteMemory value)  delete,}){
final _that = this;
switch (_that) {
case LoadMemories():
return load(_that);case RefreshMemories():
return refresh(_that);case SaveMemory():
return save(_that);case DeleteMemory():
return delete(_that);}
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>({TResult? Function( LoadMemories value)?  load,TResult? Function( RefreshMemories value)?  refresh,TResult? Function( SaveMemory value)?  save,TResult? Function( DeleteMemory value)?  delete,}){
final _that = this;
switch (_that) {
case LoadMemories() when load != null:
return load(_that);case RefreshMemories() when refresh != null:
return refresh(_that);case SaveMemory() when save != null:
return save(_that);case DeleteMemory() when delete != null:
return delete(_that);case _:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>({TResult Function()?  load,TResult Function()?  refresh,TResult Function( MemoryItem item)?  save,TResult Function( int id)?  delete,required TResult orElse(),}) {final _that = this;
switch (_that) {
case LoadMemories() when load != null:
return load();case RefreshMemories() when refresh != null:
return refresh();case SaveMemory() when save != null:
return save(_that.item);case DeleteMemory() when delete != null:
return delete(_that.id);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>({required TResult Function()  load,required TResult Function()  refresh,required TResult Function( MemoryItem item)  save,required TResult Function( int id)  delete,}) {final _that = this;
switch (_that) {
case LoadMemories():
return load();case RefreshMemories():
return refresh();case SaveMemory():
return save(_that.item);case DeleteMemory():
return delete(_that.id);}
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>({TResult? Function()?  load,TResult? Function()?  refresh,TResult? Function( MemoryItem item)?  save,TResult? Function( int id)?  delete,}) {final _that = this;
switch (_that) {
case LoadMemories() when load != null:
return load();case RefreshMemories() when refresh != null:
return refresh();case SaveMemory() when save != null:
return save(_that.item);case DeleteMemory() when delete != null:
return delete(_that.id);case _:
  return null;

}
}

}

/// @nodoc


class LoadMemories extends MemoryIntent {
  const LoadMemories(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is LoadMemories);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'MemoryIntent.load()';
}


}




/// @nodoc


class RefreshMemories extends MemoryIntent {
  const RefreshMemories(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is RefreshMemories);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'MemoryIntent.refresh()';
}


}




/// @nodoc


class SaveMemory extends MemoryIntent {
  const SaveMemory(this.item): super._();
  

 final  MemoryItem item;

/// Create a copy of MemoryIntent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SaveMemoryCopyWith<SaveMemory> get copyWith => _$SaveMemoryCopyWithImpl<SaveMemory>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SaveMemory&&(identical(other.item, item) || other.item == item));
}


@override
int get hashCode => Object.hash(runtimeType,item);

@override
String toString() {
  return 'MemoryIntent.save(item: $item)';
}


}

/// @nodoc
abstract mixin class $SaveMemoryCopyWith<$Res> implements $MemoryIntentCopyWith<$Res> {
  factory $SaveMemoryCopyWith(SaveMemory value, $Res Function(SaveMemory) _then) = _$SaveMemoryCopyWithImpl;
@useResult
$Res call({
 MemoryItem item
});




}
/// @nodoc
class _$SaveMemoryCopyWithImpl<$Res>
    implements $SaveMemoryCopyWith<$Res> {
  _$SaveMemoryCopyWithImpl(this._self, this._then);

  final SaveMemory _self;
  final $Res Function(SaveMemory) _then;

/// Create a copy of MemoryIntent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? item = null,}) {
  return _then(SaveMemory(
null == item ? _self.item : item // ignore: cast_nullable_to_non_nullable
as MemoryItem,
  ));
}


}

/// @nodoc


class DeleteMemory extends MemoryIntent {
  const DeleteMemory(this.id): super._();
  

 final  int id;

/// Create a copy of MemoryIntent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$DeleteMemoryCopyWith<DeleteMemory> get copyWith => _$DeleteMemoryCopyWithImpl<DeleteMemory>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is DeleteMemory&&(identical(other.id, id) || other.id == id));
}


@override
int get hashCode => Object.hash(runtimeType,id);

@override
String toString() {
  return 'MemoryIntent.delete(id: $id)';
}


}

/// @nodoc
abstract mixin class $DeleteMemoryCopyWith<$Res> implements $MemoryIntentCopyWith<$Res> {
  factory $DeleteMemoryCopyWith(DeleteMemory value, $Res Function(DeleteMemory) _then) = _$DeleteMemoryCopyWithImpl;
@useResult
$Res call({
 int id
});




}
/// @nodoc
class _$DeleteMemoryCopyWithImpl<$Res>
    implements $DeleteMemoryCopyWith<$Res> {
  _$DeleteMemoryCopyWithImpl(this._self, this._then);

  final DeleteMemory _self;
  final $Res Function(DeleteMemory) _then;

/// Create a copy of MemoryIntent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? id = null,}) {
  return _then(DeleteMemory(
null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}

// dart format on
