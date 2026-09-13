import 'package:flutter_riverpod/flutter_riverpod.dart';

/// 画面から ViewModel に渡す操作を表す共通インターフェース。
abstract interface class MviIntent {}

/// 画面の操作を [dispatch] に集約し、描画に必要な情報を state で公開する。
/// 非同期処理後は mounted を確認し、画面破棄やセッション切替後の更新を防ぐ。
abstract class MviViewModel<S, I extends MviIntent> extends StateNotifier<S> {
  MviViewModel(super.state);

  Future<void> dispatch(I intent);
}
