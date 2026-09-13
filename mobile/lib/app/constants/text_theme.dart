import 'package:flutter/material.dart';
import 'color_theme.dart';

@immutable
class AppTextStyles {
  const AppTextStyles._(this._context);
  final BuildContext _context;

  static AppTextStyles of(BuildContext context) => AppTextStyles._(context);

  TextTheme get _theme => Theme.of(_context).textTheme;

  TextStyle get displayLarge => _theme.displayLarge!;
  TextStyle get displayMedium => _theme.displayMedium!;
  TextStyle get displaySmall => _theme.displaySmall!;
  TextStyle get headlineLarge => _theme.headlineLarge!;
  TextStyle get headlineMedium => _theme.headlineMedium!;
  TextStyle get headlineSmall => _theme.headlineSmall!;
  TextStyle get titleLarge => _theme.titleLarge!;
  TextStyle get titleMedium => _theme.titleMedium!;
  TextStyle get titleSmall => _theme.titleSmall!;
  TextStyle get bodyLarge => _theme.bodyLarge!;
  TextStyle get bodyMedium => _theme.bodyMedium!;
  TextStyle get bodySmall => _theme.bodySmall!;
  TextStyle get labelLarge => _theme.labelLarge!;
  TextStyle get labelMedium => _theme.labelMedium!;
  TextStyle get labelSmall => _theme.labelSmall!;

  static TextTheme build(AppColors colors) {
    return TextTheme(
      displayLarge: TextStyle(
        fontSize: 32,
        fontWeight: FontWeight.bold,
        color: colors.text.primary,
      ),

      headlineLarge: TextStyle(
        fontSize: 24,
        fontWeight: FontWeight.w700,
        color: colors.text.primary,
      ),

      titleLarge: TextStyle(
        fontSize: 20,
        fontWeight: FontWeight.w600,
        color: colors.text.primary,
      ),

      bodyLarge: TextStyle(
        fontSize: 16,
        fontWeight: FontWeight.w400,
        color: colors.text.primary,
      ),

      bodyMedium: TextStyle(
        fontSize: 14,
        fontWeight: FontWeight.w400,
        color: colors.text.primary,
      ),

      bodySmall: TextStyle(
        fontSize: 12,
        fontWeight: FontWeight.w400,
        color: colors.text.secondary,
      ),

      labelLarge: TextStyle(
        fontSize: 14,
        fontWeight: FontWeight.w600,
        color: colors.text.primary,
      ),
    );
  }
}
