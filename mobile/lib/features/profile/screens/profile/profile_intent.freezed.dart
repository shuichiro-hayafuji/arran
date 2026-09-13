// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'profile_intent.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$ProfileIntent {





@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ProfileIntent);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'ProfileIntent()';
}


}

/// @nodoc
class $ProfileIntentCopyWith<$Res>  {
$ProfileIntentCopyWith(ProfileIntent _, $Res Function(ProfileIntent) __);
}


/// Adds pattern-matching-related methods to [ProfileIntent].
extension ProfileIntentPatterns on ProfileIntent {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>({TResult Function( LoadProfile value)?  load,TResult Function( SaveProfile value)?  save,required TResult orElse(),}){
final _that = this;
switch (_that) {
case LoadProfile() when load != null:
return load(_that);case SaveProfile() when save != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>({required TResult Function( LoadProfile value)  load,required TResult Function( SaveProfile value)  save,}){
final _that = this;
switch (_that) {
case LoadProfile():
return load(_that);case SaveProfile():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>({TResult? Function( LoadProfile value)?  load,TResult? Function( SaveProfile value)?  save,}){
final _that = this;
switch (_that) {
case LoadProfile() when load != null:
return load(_that);case SaveProfile() when save != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>({TResult Function()?  load,TResult Function( Profile profile)?  save,required TResult orElse(),}) {final _that = this;
switch (_that) {
case LoadProfile() when load != null:
return load();case SaveProfile() when save != null:
return save(_that.profile);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>({required TResult Function()  load,required TResult Function( Profile profile)  save,}) {final _that = this;
switch (_that) {
case LoadProfile():
return load();case SaveProfile():
return save(_that.profile);}
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>({TResult? Function()?  load,TResult? Function( Profile profile)?  save,}) {final _that = this;
switch (_that) {
case LoadProfile() when load != null:
return load();case SaveProfile() when save != null:
return save(_that.profile);case _:
  return null;

}
}

}

/// @nodoc


class LoadProfile extends ProfileIntent {
  const LoadProfile(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is LoadProfile);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'ProfileIntent.load()';
}


}




/// @nodoc


class SaveProfile extends ProfileIntent {
  const SaveProfile(this.profile): super._();
  

 final  Profile profile;

/// Create a copy of ProfileIntent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SaveProfileCopyWith<SaveProfile> get copyWith => _$SaveProfileCopyWithImpl<SaveProfile>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SaveProfile&&(identical(other.profile, profile) || other.profile == profile));
}


@override
int get hashCode => Object.hash(runtimeType,profile);

@override
String toString() {
  return 'ProfileIntent.save(profile: $profile)';
}


}

/// @nodoc
abstract mixin class $SaveProfileCopyWith<$Res> implements $ProfileIntentCopyWith<$Res> {
  factory $SaveProfileCopyWith(SaveProfile value, $Res Function(SaveProfile) _then) = _$SaveProfileCopyWithImpl;
@useResult
$Res call({
 Profile profile
});




}
/// @nodoc
class _$SaveProfileCopyWithImpl<$Res>
    implements $SaveProfileCopyWith<$Res> {
  _$SaveProfileCopyWithImpl(this._self, this._then);

  final SaveProfile _self;
  final $Res Function(SaveProfile) _then;

/// Create a copy of ProfileIntent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? profile = null,}) {
  return _then(SaveProfile(
null == profile ? _self.profile : profile // ignore: cast_nullable_to_non_nullable
as Profile,
  ));
}


}

// dart format on
