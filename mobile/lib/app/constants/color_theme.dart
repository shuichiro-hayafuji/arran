import 'package:flutter/material.dart';

import 'app_color_palette.dart';

@immutable
class AppColors {
  const AppColors._({
    required BrandColors brand,
    required BackgroundColors background,
    required BorderColors border,
    required TextColors text,
  }) : _brand = brand,
       _background = background,
       _border = border,
       _text = text;

  static const standard = AppColors._(
    brand: BrandColors._(primary: AppColorPalette.blue500),
    background: BackgroundColors._(
      primary: AppColorPalette.neutral0,
      secondary: AppColorPalette.neutral50,
      elevated: AppColorPalette.neutral0,
      disabled: AppColorPalette.neutral100,
    ),
    border: BorderColors._(
      primary: AppColorPalette.neutral300,
      secondary: AppColorPalette.neutral200,
      focused: AppColorPalette.blue500,
    ),
    text: TextColors._(
      primary: AppColorPalette.neutral900,
      secondary: AppColorPalette.neutral700,
      tertiary: AppColorPalette.neutral500,
      disabled: AppColorPalette.neutral500,
    ),
  );

  final BrandColors _brand;
  final BackgroundColors _background;
  final BorderColors _border;
  final TextColors _text;

  BrandColors get brand => _brand;
  BackgroundColors get background => _background;
  BorderColors get border => _border;
  TextColors get text => _text;
}

@immutable
class BrandColors {
  const BrandColors._({required this.primary});

  final Color primary;
}

@immutable
class BackgroundColors {
  const BackgroundColors._({
    required this.primary,
    required this.secondary,
    required this.elevated,
    required this.disabled,
  });

  final Color primary;
  final Color secondary;
  final Color elevated;
  final Color disabled;
}

@immutable
class BorderColors {
  const BorderColors._({
    required this.primary,
    required this.secondary,
    required this.focused,
  });

  final Color primary;
  final Color secondary;
  final Color focused;
}

@immutable
class TextColors {
  const TextColors._({
    required this.primary,
    required this.secondary,
    required this.tertiary,
    required this.disabled,
  });

  final Color primary;
  final Color secondary;
  final Color tertiary;
  final Color disabled;
}
