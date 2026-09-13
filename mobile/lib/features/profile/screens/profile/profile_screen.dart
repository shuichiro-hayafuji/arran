import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:spendable_today/features/profile/domain/profile.dart';
import 'package:spendable_today/features/profile/screens/profile/profile_intent.dart';
import 'package:spendable_today/features/profile/screens/profile/profile_view_model.dart';

const reduceCategoryOptions = [
  '酒・飲み会',
  'サブスクリプション',
  'コンビニ',
  '衝動買い',
  '外食',
  '娯楽',
  'タクシー',
  'その他',
];

const allowedCategoryOptions = ['学習', '健康', '仕事', '人間関係', '旅行', '趣味', 'その他'];

String? validateProfileValues({
  required String monthlyIncome,
  required String currentBalance,
  required String monthlyFixedCosts,
  required String monthlyFreeBudget,
  required String monthlySavingsGoal,
  required String strictness,
}) {
  final values = [
    monthlyIncome,
    currentBalance,
    monthlyFixedCosts,
    monthlyFreeBudget,
    monthlySavingsGoal,
  ].map(int.tryParse).toList();
  if (values.any((value) => value == null || value < 0)) {
    return '金額は0円以上の整数で入力してください。';
  }
  if (values[3] == 0) {
    return '月間の自由支出予算は必須です。';
  }
  if (!const ['やさしい', 'バランス型', '厳しい'].contains(strictness)) {
    return 'AIの厳しさを選択してください。';
  }
  return null;
}

class ProfileScreen extends ConsumerStatefulWidget {
  const ProfileScreen({required this.onboarding, super.key});

  final bool onboarding;

  @override
  ConsumerState<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends ConsumerState<ProfileScreen> {
  final _income = TextEditingController();
  final _balance = TextEditingController();
  final _fixed = TextEditingController();
  final _free = TextEditingController();
  final _savings = TextEditingController();
  final _goal = TextEditingController();
  final _reduce = <String>{};
  final _allowed = <String>{};
  var _strictness = 'バランス型';
  var _hydrated = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    if (widget.onboarding) {
      _income.text = '400000';
      _balance.text = '1000000';
      _fixed.text = '180000';
      _free.text = '60000';
      _savings.text = '80000';
      _reduce.addAll(['酒・飲み会', 'サブスクリプション']);
      _allowed.addAll(['学習', '健康']);
      _hydrated = true;
    }
  }

  @override
  void dispose() {
    _income.dispose();
    _balance.dispose();
    _fixed.dispose();
    _free.dispose();
    _savings.dispose();
    _goal.dispose();
    super.dispose();
  }

  void _hydrate(Profile? profile) {
    if (profile == null) return;
    if (_hydrated) return;
    _hydrated = true;
    _income.text = '${profile.monthlyIncome}';
    _balance.text = '${profile.currentBalance}';
    _fixed.text = '${profile.monthlyFixedCosts}';
    _free.text = '${profile.monthlyFreeBudget}';
    _savings.text = '${profile.monthlySavingsGoal}';
    _goal.text = profile.longTermGoal;
    _reduce.addAll(profile.reduceCategories);
    _allowed.addAll(profile.allowedCategories);
    _strictness = profile.adviceStrictness;
  }

  Future<void> _save() async {
    final validation = validateProfileValues(
      monthlyIncome: _income.text,
      currentBalance: _balance.text,
      monthlyFixedCosts: _fixed.text,
      monthlyFreeBudget: _free.text,
      monthlySavingsGoal: _savings.text,
      strictness: _strictness,
    );
    if (validation != null) {
      setState(() => _error = validation);
      return;
    }
    setState(() => _error = null);
    final profile = Profile(
      monthlyIncome: int.parse(_income.text),
      currentBalance: int.parse(_balance.text),
      monthlyFixedCosts: int.parse(_fixed.text),
      monthlyFreeBudget: int.parse(_free.text),
      monthlySavingsGoal: int.parse(_savings.text),
      reduceCategories: _reduce.toList(),
      allowedCategories: _allowed.toList(),
      longTermGoal: _goal.text.trim(),
      adviceStrictness: _strictness,
    );
    await ref
        .read(profileViewModelProvider(widget.onboarding).notifier)
        .dispatch(SaveProfile(profile));
  }

  @override
  Widget build(BuildContext context) {
    final profileState = ref.watch(profileViewModelProvider(widget.onboarding));
    profileState.profile.whenData(_hydrate);
    ref.listen(profileViewModelProvider(widget.onboarding), (previous, next) {
      if (next.saved && previous?.saved != true && context.mounted) {
        context.go('/home');
      }
      if (next.actionError != null &&
          next.actionError != previous?.actionError &&
          context.mounted) {
        setState(() => _error = next.actionError.toString());
      }
    });
    return Scaffold(
      appBar: AppBar(title: Text(widget.onboarding ? '最初のプロフィール' : 'プロフィール編集')),
      body: SafeArea(
        child: (!_hydrated && profileState.profile.isLoading)
            ? const Center(child: CircularProgressIndicator())
            : ListView(
                padding: const EdgeInsets.fromLTRB(20, 8, 20, 36),
                children: [
                  Text(
                    widget.onboarding ? 'まず、判断の軸を教えてください' : '予算と価値観を更新する',
                    style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                  const SizedBox(height: 8),
                  const Text('あとからいつでも変更できます。すべて整数の円単位です。'),
                  const SizedBox(height: 20),
                  _MoneyField(label: '月収', controller: _income),
                  _MoneyField(label: '現在の預金残高', controller: _balance),
                  _MoneyField(label: '毎月の固定費', controller: _fixed),
                  _MoneyField(
                    key: const ValueKey('monthly-free-budget'),
                    label: '月間の自由支出予算（必須）',
                    controller: _free,
                  ),
                  _MoneyField(label: '毎月の貯蓄目標', controller: _savings),
                  const SizedBox(height: 8),
                  const Text(
                    '減らしたいカテゴリ',
                    style: TextStyle(fontWeight: FontWeight.w800),
                  ),
                  const SizedBox(height: 8),
                  Wrap(
                    spacing: 7,
                    children: [
                      for (final option in reduceCategoryOptions)
                        FilterChip(
                          label: Text(option),
                          selected: _reduce.contains(option),
                          onSelected: (selected) => setState(() {
                            selected
                                ? _reduce.add(option)
                                : _reduce.remove(option);
                          }),
                        ),
                    ],
                  ),
                  const SizedBox(height: 16),
                  const Text(
                    'お金を使ってよいカテゴリ',
                    style: TextStyle(fontWeight: FontWeight.w800),
                  ),
                  const SizedBox(height: 8),
                  Wrap(
                    spacing: 7,
                    children: [
                      for (final option in allowedCategoryOptions)
                        FilterChip(
                          label: Text(option),
                          selected: _allowed.contains(option),
                          onSelected: (selected) => setState(() {
                            selected
                                ? _allowed.add(option)
                                : _allowed.remove(option);
                          }),
                        ),
                    ],
                  ),
                  const SizedBox(height: 16),
                  TextField(
                    controller: _goal,
                    minLines: 2,
                    maxLines: 4,
                    decoration: const InputDecoration(
                      labelText: '中長期的な目標',
                      hintText: '例：1年で100万円貯める',
                    ),
                  ),
                  const SizedBox(height: 16),
                  DropdownButtonFormField<String>(
                    initialValue: _strictness,
                    decoration: const InputDecoration(labelText: 'AIの厳しさ'),
                    items: const [
                      DropdownMenuItem(value: 'やさしい', child: Text('やさしい')),
                      DropdownMenuItem(value: 'バランス型', child: Text('バランス型')),
                      DropdownMenuItem(value: '厳しい', child: Text('厳しい')),
                    ],
                    onChanged: (value) {
                      if (value != null) setState(() => _strictness = value);
                    },
                  ),
                  if (_error != null) ...[
                    const SizedBox(height: 12),
                    Text(
                      _error!,
                      key: const ValueKey('profile-error'),
                      style: TextStyle(
                        color: Theme.of(context).colorScheme.error,
                      ),
                    ),
                  ],
                  const SizedBox(height: 20),
                  FilledButton(
                    key: const ValueKey('save-profile'),
                    onPressed: profileState.saving ? null : _save,
                    child: Padding(
                      padding: const EdgeInsets.symmetric(vertical: 14),
                      child: Text(profileState.saving ? '保存中…' : 'プロフィールを保存'),
                    ),
                  ),
                ],
              ),
      ),
    );
  }
}

class _MoneyField extends StatelessWidget {
  const _MoneyField({required this.label, required this.controller, super.key});

  final String label;
  final TextEditingController controller;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: TextField(
        controller: controller,
        keyboardType: TextInputType.number,
        inputFormatters: [FilteringTextInputFormatter.digitsOnly],
        decoration: InputDecoration(labelText: label, suffixText: '円'),
      ),
    );
  }
}
