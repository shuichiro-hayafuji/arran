import 'package:flutter/material.dart';

import 'color_theme.dart';
import 'text_theme.dart';

abstract final class AppTheme {
  static ThemeData get standard {
    const colors = AppColors.standard;

    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.light,

      colorScheme: ColorScheme.fromSeed(
        seedColor: colors.brand.primary,
        brightness: Brightness.light,
      ),

      scaffoldBackgroundColor: colors.background.primary,

      textTheme: AppTextStyles.build(colors),

      dividerTheme: DividerThemeData(
        color: colors.border.primary,
        thickness: 1,
        space: 1,
      ),

      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: colors.background.secondary,
        enabledBorder: OutlineInputBorder(
          borderSide: BorderSide(color: colors.border.primary),
        ),
        focusedBorder: OutlineInputBorder(
          borderSide: BorderSide(color: colors.border.focused),
        ),
      ),
    );
  }
}
