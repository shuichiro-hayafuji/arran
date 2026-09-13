// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'dashboard_intent.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$DashboardIntent {





@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is DashboardIntent);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'DashboardIntent()';
}


}

/// @nodoc
class $DashboardIntentCopyWith<$Res>  {
$DashboardIntentCopyWith(DashboardIntent _, $Res Function(DashboardIntent) __);
}


/// Adds pattern-matching-related methods to [DashboardIntent].
extension DashboardIntentPatterns on DashboardIntent {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>({TResult Function( LoadDashboard value)?  load,TResult Function( RefreshDashboard value)?  refresh,required TResult orElse(),}){
final _that = this;
switch (_that) {
case LoadDashboard() when load != null:
return load(_that);case RefreshDashboard() when refresh != null:
return refresh(_that);case _:
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

@optionalTypeArgs TResult map<TResult extends Object?>({required TResult Function( LoadDashboard value)  load,required TResult Function( RefreshDashboard value)  refresh,}){
final _that = this;
switch (_that) {
case LoadDashboard():
return load(_that);case RefreshDashboard():
return refresh(_that);}
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>({TResult? Function( LoadDashboard value)?  load,TResult? Function( RefreshDashboard value)?  refresh,}){
final _that = this;
switch (_that) {
case LoadDashboard() when load != null:
return load(_that);case RefreshDashboard() when refresh != null:
return refresh(_that);case _:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>({TResult Function()?  load,TResult Function()?  refresh,required TResult orElse(),}) {final _that = this;
switch (_that) {
case LoadDashboard() when load != null:
return load();case RefreshDashboard() when refresh != null:
return refresh();case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>({required TResult Function()  load,required TResult Function()  refresh,}) {final _that = this;
switch (_that) {
case LoadDashboard():
return load();case RefreshDashboard():
return refresh();}
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>({TResult? Function()?  load,TResult? Function()?  refresh,}) {final _that = this;
switch (_that) {
case LoadDashboard() when load != null:
return load();case RefreshDashboard() when refresh != null:
return refresh();case _:
  return null;

}
}

}

/// @nodoc


class LoadDashboard extends DashboardIntent {
  const LoadDashboard(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is LoadDashboard);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'DashboardIntent.load()';
}


}




/// @nodoc


class RefreshDashboard extends DashboardIntent {
  const RefreshDashboard(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is RefreshDashboard);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'DashboardIntent.refresh()';
}


}




// dart format on
