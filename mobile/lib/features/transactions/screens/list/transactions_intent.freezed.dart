// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'transactions_intent.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;
/// @nodoc
mixin _$TransactionsIntent {





@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is TransactionsIntent);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'TransactionsIntent()';
}


}

/// @nodoc
class $TransactionsIntentCopyWith<$Res>  {
$TransactionsIntentCopyWith(TransactionsIntent _, $Res Function(TransactionsIntent) __);
}


/// Adds pattern-matching-related methods to [TransactionsIntent].
extension TransactionsIntentPatterns on TransactionsIntent {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>({TResult Function( LoadTransactions value)?  load,TResult Function( RefreshTransactions value)?  refresh,TResult Function( ChangeTransactionCategory value)?  changeCategory,required TResult orElse(),}){
final _that = this;
switch (_that) {
case LoadTransactions() when load != null:
return load(_that);case RefreshTransactions() when refresh != null:
return refresh(_that);case ChangeTransactionCategory() when changeCategory != null:
return changeCategory(_that);case _:
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

@optionalTypeArgs TResult map<TResult extends Object?>({required TResult Function( LoadTransactions value)  load,required TResult Function( RefreshTransactions value)  refresh,required TResult Function( ChangeTransactionCategory value)  changeCategory,}){
final _that = this;
switch (_that) {
case LoadTransactions():
return load(_that);case RefreshTransactions():
return refresh(_that);case ChangeTransactionCategory():
return changeCategory(_that);}
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>({TResult? Function( LoadTransactions value)?  load,TResult? Function( RefreshTransactions value)?  refresh,TResult? Function( ChangeTransactionCategory value)?  changeCategory,}){
final _that = this;
switch (_that) {
case LoadTransactions() when load != null:
return load(_that);case RefreshTransactions() when refresh != null:
return refresh(_that);case ChangeTransactionCategory() when changeCategory != null:
return changeCategory(_that);case _:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>({TResult Function()?  load,TResult Function()?  refresh,TResult Function( int id,  String category,  bool rememberMerchant)?  changeCategory,required TResult orElse(),}) {final _that = this;
switch (_that) {
case LoadTransactions() when load != null:
return load();case RefreshTransactions() when refresh != null:
return refresh();case ChangeTransactionCategory() when changeCategory != null:
return changeCategory(_that.id,_that.category,_that.rememberMerchant);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>({required TResult Function()  load,required TResult Function()  refresh,required TResult Function( int id,  String category,  bool rememberMerchant)  changeCategory,}) {final _that = this;
switch (_that) {
case LoadTransactions():
return load();case RefreshTransactions():
return refresh();case ChangeTransactionCategory():
return changeCategory(_that.id,_that.category,_that.rememberMerchant);}
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>({TResult? Function()?  load,TResult? Function()?  refresh,TResult? Function( int id,  String category,  bool rememberMerchant)?  changeCategory,}) {final _that = this;
switch (_that) {
case LoadTransactions() when load != null:
return load();case RefreshTransactions() when refresh != null:
return refresh();case ChangeTransactionCategory() when changeCategory != null:
return changeCategory(_that.id,_that.category,_that.rememberMerchant);case _:
  return null;

}
}

}

/// @nodoc


class LoadTransactions extends TransactionsIntent {
  const LoadTransactions(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is LoadTransactions);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'TransactionsIntent.load()';
}


}




/// @nodoc


class RefreshTransactions extends TransactionsIntent {
  const RefreshTransactions(): super._();
  






@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is RefreshTransactions);
}


@override
int get hashCode => runtimeType.hashCode;

@override
String toString() {
  return 'TransactionsIntent.refresh()';
}


}




/// @nodoc


class ChangeTransactionCategory extends TransactionsIntent {
  const ChangeTransactionCategory(this.id, this.category, {required this.rememberMerchant}): super._();
  

 final  int id;
 final  String category;
 final  bool rememberMerchant;

/// Create a copy of TransactionsIntent
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$ChangeTransactionCategoryCopyWith<ChangeTransactionCategory> get copyWith => _$ChangeTransactionCategoryCopyWithImpl<ChangeTransactionCategory>(this, _$identity);



@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is ChangeTransactionCategory&&(identical(other.id, id) || other.id == id)&&(identical(other.category, category) || other.category == category)&&(identical(other.rememberMerchant, rememberMerchant) || other.rememberMerchant == rememberMerchant));
}


@override
int get hashCode => Object.hash(runtimeType,id,category,rememberMerchant);

@override
String toString() {
  return 'TransactionsIntent.changeCategory(id: $id, category: $category, rememberMerchant: $rememberMerchant)';
}


}

/// @nodoc
abstract mixin class $ChangeTransactionCategoryCopyWith<$Res> implements $TransactionsIntentCopyWith<$Res> {
  factory $ChangeTransactionCategoryCopyWith(ChangeTransactionCategory value, $Res Function(ChangeTransactionCategory) _then) = _$ChangeTransactionCategoryCopyWithImpl;
@useResult
$Res call({
 int id, String category, bool rememberMerchant
});




}
/// @nodoc
class _$ChangeTransactionCategoryCopyWithImpl<$Res>
    implements $ChangeTransactionCategoryCopyWith<$Res> {
  _$ChangeTransactionCategoryCopyWithImpl(this._self, this._then);

  final ChangeTransactionCategory _self;
  final $Res Function(ChangeTransactionCategory) _then;

/// Create a copy of TransactionsIntent
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') $Res call({Object? id = null,Object? category = null,Object? rememberMerchant = null,}) {
  return _then(ChangeTransactionCategory(
null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as int,null == category ? _self.category : category // ignore: cast_nullable_to_non_nullable
as String,rememberMerchant: null == rememberMerchant ? _self.rememberMerchant : rememberMerchant // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}


}

// dart format on
