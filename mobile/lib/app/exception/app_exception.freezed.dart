// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'app_exception.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$AppException {





@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AppException);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'AppException()';
}


}

/// @nodoc
class $AppExceptionCopyWith<$Res>  {
$AppExceptionCopyWith(AppException _, $Res Function(AppException) __);
}


/// Adds pattern-matching-related methods to [AppException].
extension AppExceptionPatterns on AppException {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>({TResult Function( AppExceptionError value)?  error,TResult Function( AppExceptionInitialize value)?  initialize,TResult Function( AppExceptionTimeout value)?  timeout,required TResult orElse(),}){
final _that = this;
switch (_that) {
case AppExceptionError() when error != null:
return error(_that);case AppExceptionInitialize() when initialize != null:
return initialize(_that);case AppExceptionTimeout() when timeout != null:
return timeout(_that);case _:
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

@optionalTypeArgs TResult map<TResult extends Object?>({required TResult Function( AppExceptionError value)  error,required TResult Function( AppExceptionInitialize value)  initialize,required TResult Function( AppExceptionTimeout value)  timeout,}){
final _that = this;
switch (_that) {
case AppExceptionError():
return error(_that);case AppExceptionInitialize():
return initialize(_that);case AppExceptionTimeout():
return timeout(_that);case _:
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>({TResult? Function( AppExceptionError value)?  error,TResult? Function( AppExceptionInitialize value)?  initialize,TResult? Function( AppExceptionTimeout value)?  timeout,}){
final _that = this;
switch (_that) {
case AppExceptionError() when error != null:
return error(_that);case AppExceptionInitialize() when initialize != null:
return initialize(_that);case AppExceptionTimeout() when timeout != null:
return timeout(_that);case _:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>({TResult Function()?  error,TResult Function()?  initialize,TResult Function()?  timeout,required TResult orElse(),}) {final _that = this;
switch (_that) {
case AppExceptionError() when error != null:
return error();case AppExceptionInitialize() when initialize != null:
return initialize();case AppExceptionTimeout() when timeout != null:
return timeout();case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>({required TResult Function()  error,required TResult Function()  initialize,required TResult Function()  timeout,}) {final _that = this;
switch (_that) {
case AppExceptionError():
return error();case AppExceptionInitialize():
return initialize();case AppExceptionTimeout():
return timeout();case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>({TResult? Function()?  error,TResult? Function()?  initialize,TResult? Function()?  timeout,}) {final _that = this;
switch (_that) {
case AppExceptionError() when error != null:
return error();case AppExceptionInitialize() when initialize != null:
return initialize();case AppExceptionTimeout() when timeout != null:
return timeout();case _:
  return null;

}
}

}

/// @nodoc


class AppExceptionError implements AppException {
  const AppExceptionError();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AppExceptionError);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'AppException.error()';
}


}




/// @nodoc


class AppExceptionInitialize implements AppException {
  const AppExceptionInitialize();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AppExceptionInitialize);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'AppException.initialize()';
}


}




/// @nodoc


class AppExceptionTimeout implements AppException {
  const AppExceptionTimeout();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AppExceptionTimeout);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'AppException.timeout()';
}


}




// dart format on
