// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'startup_intent.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$StartupIntent {





@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is StartupIntent);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'StartupIntent()';
}


}

/// @nodoc
class $StartupIntentCopyWith<$Res>  {
$StartupIntentCopyWith(StartupIntent _, $Res Function(StartupIntent) __);
}


/// Adds pattern-matching-related methods to [StartupIntent].
extension StartupIntentPatterns on StartupIntent {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>({TResult Function( InitializeStartup value)?  initialize,TResult Function( RetryStartup value)?  retry,required TResult orElse(),}){
final _that = this;
switch (_that) {
case InitializeStartup() when initialize != null:
return initialize(_that);case RetryStartup() when retry != null:
return retry(_that);case _:
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

@optionalTypeArgs TResult map<TResult extends Object?>({required TResult Function( InitializeStartup value)  initialize,required TResult Function( RetryStartup value)  retry,}){
final _that = this;
switch (_that) {
case InitializeStartup():
return initialize(_that);case RetryStartup():
return retry(_that);}
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>({TResult? Function( InitializeStartup value)?  initialize,TResult? Function( RetryStartup value)?  retry,}){
final _that = this;
switch (_that) {
case InitializeStartup() when initialize != null:
return initialize(_that);case RetryStartup() when retry != null:
return retry(_that);case _:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>({TResult Function()?  initialize,TResult Function()?  retry,required TResult orElse(),}) {final _that = this;
switch (_that) {
case InitializeStartup() when initialize != null:
return initialize();case RetryStartup() when retry != null:
return retry();case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>({required TResult Function()  initialize,required TResult Function()  retry,}) {final _that = this;
switch (_that) {
case InitializeStartup():
return initialize();case RetryStartup():
return retry();}
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>({TResult? Function()?  initialize,TResult? Function()?  retry,}) {final _that = this;
switch (_that) {
case InitializeStartup() when initialize != null:
return initialize();case RetryStartup() when retry != null:
return retry();case _:
  return null;

}
}

}

/// @nodoc


class InitializeStartup extends StartupIntent {
  const InitializeStartup(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is InitializeStartup);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'StartupIntent.initialize()';
}


}




/// @nodoc


class RetryStartup extends StartupIntent {
  const RetryStartup(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is RetryStartup);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'StartupIntent.retry()';
}


}




// dart format on
